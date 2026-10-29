-- A device that wrote items is signed out like any other: which device
-- last wrote an item is a note, not a reference that keeps it alive.
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_updated_by_fkey;
