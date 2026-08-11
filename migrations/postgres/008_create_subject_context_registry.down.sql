ALTER TABLE analytics_event_subjects DROP CONSTRAINT IF EXISTS fk_event_subjects_context_revision;
ALTER TABLE analytics_event_subjects DROP CONSTRAINT IF EXISTS fk_event_subjects_context_definition;
DROP TABLE IF EXISTS analytics_subject_context_daily;
DROP TABLE IF EXISTS analytics_unregistered_contexts;
DROP TABLE IF EXISTS analytics_context_registry_entries;
DROP TABLE IF EXISTS analytics_context_definitions;
DROP TABLE IF EXISTS analytics_context_registry_revisions;
