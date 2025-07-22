rename table clickcomments to clickcomments_3_vec;
drop table clickcomments;

CREATE TABLE clickcomments
(
    number        UInt32,
    title         String,
    state         Enum8('none' = 0, 'open' = 1, 'closed' = 2),
    labels        Array(String),
    created_at    DateTime,
    updated_at    DateTime,
    composite_vec Array(Float32) CODEC(NONE) ,
    INDEX idx_composite composite_vec TYPE vector_similarity('hnsw','cosineDistance',1536)
)
ENGINE = MergeTree ORDER BY (number)
settings allow_experimental_vector_similarity_index=1;

select count() from clickcomments;
select length(composite_vec),* from clickcomments;

select formatReadableSize(sum(bytes_on_disk) as s) as f
from system.parts where active and table='clickcomments';

select column, formatReadableSize(sum(column_bytes_on_disk) as s) as f
from system.parts_columns where active and table='clickcomments' group by all order by s desc;

