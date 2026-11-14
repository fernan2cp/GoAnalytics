-- ADVERTENCIA: este down elimina sujetos, asociaciones normalizadas y el
-- progreso de bootstrap. Solo debe ejecutarse en desarrollo o con autorización
-- explícita de pérdida de datos.
DROP TABLE IF EXISTS analytics_subject_bootstrap;
DROP TABLE IF EXISTS analytics_event_subjects;
DROP TABLE IF EXISTS analytics_subjects;
