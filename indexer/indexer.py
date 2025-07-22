#!/usr/bin/env python3
"""
Embed clickissues.title and comment into clickcomments with two vector columns:
- title_vec
- comment_vec

The table must define a MATERIALIZED column `composite_vec`, so it will be computed inside ClickHouse.
"""

import os
import xml.etree.ElementTree as ET
import numpy as np
from typing import List
import ssl
import certifi
import argparse
import math
import httpx
import certifi
os.environ["REQUESTS_CA_BUNDLE"] = certifi.where()

from clickhouse_driver import Client
from sentence_transformers import SentenceTransformer
import openai
try:
    import tiktoken
except ImportError:
    tiktoken = None  # token counting will fall back to rough heuristic

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------

MODEL_NAME = "sentence-transformers/all-MiniLM-L6-v2"  # 384d, fast & compact
BATCH_SIZE = 64

parser = argparse.ArgumentParser(description="Embed GitHub comments into ClickHouse")
parser.add_argument("--model", choices=["openai", "local"], default="openai",
                    help="Embedding model to use (default: local)")
args = parser.parse_args()

# -----------------------------------------------------------------------------
# Model loading and setup
# -----------------------------------------------------------------------------

if args.model == "local":
    print(f"Loading model: {MODEL_NAME}")
    model = SentenceTransformer(MODEL_NAME)
    model.max_seq_length = 512  # truncate long texts
    print("Model loaded and ready.")
else:
    model = None

# -----------------------------------------------------------------------------
# Embedding function
# -----------------------------------------------------------------------------

def embed_batch_openai(texts: List[str]) -> List[List[float]]:
    """
    Call OpenAI to embed a list of strings with text-embedding-3-small.
    Splits into sub-batches of ≤ 96 items (OpenAI limit).
    Returns a list of 1536-d float vectors.
    """
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("OPENAI_API_KEY env var is not set")

    client = openai.OpenAI(
        api_key=api_key,
        http_client=httpx.Client(verify=certifi.where())
    )

    # --- sanitize inputs: OpenAI rejects empty strings in the list ---
    # Replace any None or whitespace‑only text with a single period so the
    # output list length is preserved.
    texts = [
        "." if (t is None or str(t).strip() == "") else str(t)
        for t in texts
    ]

    MAX_ITEMS = 96
    MAX_TOKENS = 8192  # hard limit from OpenAI

    # Because our local token count can differ slightly from the server’s,
    # leave ~300‑token headroom so the request never trips the 8192 cap.
    EFFECTIVE_MAX_TOKENS = 7900

    # Choose a tokenizer for precise token counting if available
    if tiktoken is not None:
        try:
            encoding = tiktoken.encoding_for_model("text-embedding-3-small")
        except Exception:
            encoding = tiktoken.get_encoding("cl100k_base")
        def count_tokens(s: str) -> int:
            return len(encoding.encode(s))
    else:
        # Fallback heuristic: 3 chars ≈ 1 token
        def count_tokens(s: str) -> int:
            return max(1, (len(s) + 2) // 3)

    batches: list[list[str]] = []
    current_batch: list[str] = []
    current_tokens = 0

    for txt in texts:
        token_len = count_tokens(txt)
        # if single input is too large, truncate the text to fit
        if token_len > EFFECTIVE_MAX_TOKENS:
            # precisely truncate to fit
            if tiktoken is not None:
                ids = encoding.encode(txt)[:EFFECTIVE_MAX_TOKENS]
                txt = encoding.decode(ids)
                token_len = len(ids)
            else:
                txt = txt[:EFFECTIVE_MAX_TOKENS * 3]    # heuristic fallback
                token_len = count_tokens(txt)

        # flush the current batch if needed
        if (len(current_batch) >= MAX_ITEMS) or (current_tokens + token_len > EFFECTIVE_MAX_TOKENS):
            batches.append(current_batch)
            current_batch = []
            current_tokens = 0

        current_batch.append(txt)
        current_tokens += token_len

    if current_batch:
        batches.append(current_batch)

    all_vecs: list[list[float]] = []
    for chunk in batches:
        resp = client.embeddings.create(
            model="text-embedding-3-small",
            input=chunk
        )
        all_vecs.extend([item.embedding for item in resp.data])

    return all_vecs

def embed_batch(texts: List[str]) -> List[List[float]]:
    """
    Batch embed list of strings. Returns list of 384-dim float32 lists.
    """
    vectors = model.encode(
        texts,
        batch_size=BATCH_SIZE,
        convert_to_numpy=True,
        normalize_embeddings=True,
        show_progress_bar=False
    )
    return [vec.astype(np.float32).tolist() for vec in vectors]

CONFIG_FILE = os.path.expanduser("~/.clickhouse-client/config.xml")
CONNECTION_NAME = "github"

def load_clickhouse_config(connection_name):
    tree = ET.parse(CONFIG_FILE)
    root = tree.getroot()
    for conn in root.find("connections_credentials"):
        name_el = conn.find("name")
        if name_el is not None and name_el.text == connection_name:
            return {
                "host": conn.find("hostname").text,
                "port": int(conn.find("port").text),
                "username": conn.find("user").text,
                "password": conn.find("password").text,
                "database": conn.find("database").text,
                "secure": conn.find("secure").text == "1"
            }
    raise ValueError(f"Connection '{connection_name}' not found in config.")

ch_config = load_clickhouse_config(CONNECTION_NAME)

print("Loaded ClickHouse config:")
for key, val in ch_config.items():
    if key == "password":
        val = "***"
    print(f"  {key}: {val}")

print("Connecting to ClickHouse via TCP…")
try:
    ch = Client(
        host=ch_config["host"],
        port=ch_config["port"],
        user=ch_config["username"],
        password=ch_config["password"],
        database=ch_config["database"],
        secure=ch_config["secure"]
    )
    print("ClickHouse TCP client initialized.")
except Exception as e:
    print("Failed to connect to ClickHouse via TCP:")
    print(e)
    raise


query = """
select number,
       any(title) as title_text,
       min(created_at) created_at,
       max(updated_at) updated_at,
       max(state) state,
       arrayDistinct(arrayFlatten(groupArray(labels))) labels,
       any(title) || ' '|| any(title) || '\n'|| arrayStringConcat( groupArray(body),' ') as text
from (
    SELECT number,created_at, updated_at, state,labels,
        title, body, actor_login
    FROM github_events
    WHERE repo_name = 'ClickHouse/ClickHouse'
    ORDER BY updated_at DESC
    LIMIT 1 BY (number, comment_id)
    )
group by all
"""

print("Fetching source rows…")
rows = ch.execute(query)
print(f"Fetched {len(rows):,} rows.")

# -----------------------------------------------------------------------------
# Batch processing and insert
# -----------------------------------------------------------------------------

def grouper(iterable, n):
    batch = []
    for item in iterable:
        batch.append(item)
        if len(batch) == n:
            yield batch
            batch = []
    if batch:
        yield batch

INSERT_COLUMNS = [
    "number",
    "state",
    "labels",
    "created_at",
    "updated_at",
    "composite_vec",
    "title"
]

total = 0
for batch in grouper(rows, BATCH_SIZE):
    numbers, titles, created_list, updated_list, states, labels_list, texts = zip(*batch)

    # Embed the aggregated text once per row
    text_vecs = embed_batch_openai(texts) if args.model == "openai" else embed_batch(texts)

    # Build rows for the new schema
    insert_rows = [
        [num, state, lbls, created_at, updated_at, vec, title]
        for num, created_at, updated_at, state, lbls, vec, title in zip(
            numbers, created_list, updated_list, states, labels_list, text_vecs, titles
        )
    ]

    ch.execute(
        "INSERT INTO clickcomments (" + ",".join(INSERT_COLUMNS) + ") VALUES",
        insert_rows
    )
    total += len(insert_rows)
    print(f"Inserted {total:,} rows…", end="\r")

print(f"\nDone. {total:,} rows written.")