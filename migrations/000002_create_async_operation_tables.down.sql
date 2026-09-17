BEGIN;

DROP TABLE public.audit_logs;
DROP TABLE public.outbox_messages;
DROP TABLE public.operations;

COMMIT;