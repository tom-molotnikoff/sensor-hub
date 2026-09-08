-- The up-migration reconciles rows against readings. Which rows it removed is not
-- recoverable afterwards, and the rows it added are indistinguishable from rows that
-- were already correct, so there is nothing to undo.
SELECT 1;
