-- Rules the database enforces, so that they hold whatever writes to it: a bug, a
-- script, or a person with psql. This file reruns whenever it changes.

-- Every entry balances. A deferred trigger waits for the commit, so an entry's lines
-- can arrive one by one.
CREATE OR REPLACE FUNCTION assert_entry_balances() RETURNS trigger AS $$
DECLARE
    total bigint;
BEGIN
    SELECT coalesce(sum(amount), 0) INTO total FROM lines WHERE entry_id = NEW.entry_id;
    IF total <> 0 THEN
        RAISE EXCEPTION 'entry % does not balance: off by %', NEW.entry_id, total USING ERRCODE = 'check_violation';
    END IF;
    RETURN NULL;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS lines_balance ON lines;
CREATE CONSTRAINT TRIGGER lines_balance AFTER INSERT ON lines
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION assert_entry_balances();

-- A posted line never changes. Corrections are new entries.
CREATE OR REPLACE FUNCTION refuse_change() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = 'restrict_violation';
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS lines_append_only ON lines;
CREATE TRIGGER lines_append_only BEFORE UPDATE OR DELETE ON lines
    FOR EACH ROW EXECUTE FUNCTION refuse_change();
