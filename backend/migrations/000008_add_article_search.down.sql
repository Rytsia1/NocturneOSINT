DROP INDEX IF EXISTS articles_search_idx;
ALTER TABLE articles DROP COLUMN IF EXISTS search_vector;
