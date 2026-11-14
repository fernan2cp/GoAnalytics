"""Certifica el MVP con procesos reales y bases efímeras, sin datos de negocio.

Requiere CA_CERT_POSTGRES_DSN, CA_CERT_REDIS_ADDR y --dragon-root. El DSN debe
apuntar a PostgreSQL local de pruebas con permiso CREATE DATABASE. Nunca modifica
la base indicada: crea y elimina exclusivamente una base ca_cert_<uuid> propia.
"""
from __future__ import annotations

import argparse
import asyncio
from datetime import datetime, timezone
import importlib.util
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import sys
import tempfile
import threading
import time
from types import SimpleNamespace
from uuid import uuid4

import httpx
import psycopg2
from psycopg2 import sql
import redis
from sqlalchemy import create_engine, text
from sqlalchemy.orm import Session


def free_port():
    """Reserva un puerto efímero para los procesos de prueba locales."""
    with socket.socket() as value:
        value.bind(("127.0.0.1", 0))
        return value.getsockname()[1]


def wait_for(check, description, timeout=30):
    """Espera una condición observable con límite y conserva el último error."""
    deadline, last = time.monotonic() + timeout, None
    while time.monotonic() < deadline:
        try:
            result = check()
            if result:
                return result
        except Exception as error:
            last = error
        time.sleep(0.1)
    raise AssertionError(f"No se completó {description}: {last}")


def load_module(name, path):
    """Carga un borde concreto sin importar los routers ajenos del monorepo."""
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def certify(dragon_root):
    """Ejecuta emisión → HTTP → Redis → worker → SQL → query → hidratación."""
    root = Path(__file__).resolve().parents[1]
    sys.path[:0] = [str(dragon_root / "backend/df_av_back"), str(dragon_root / "backend")]
    from fastapi import FastAPI
    import uvicorn
    from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession
    from adapters.platform.subject_analytics import snapshot as source
    from services.analytics import subject_events, subject_suggestions
    from services.analytics.subject_snapshot import SubjectSnapshotStore
    from core.config import settings

    admin_dsn = os.environ["CA_CERT_POSTGRES_DSN"]
    admin = psycopg2.connect(admin_dsn)
    if admin.get_dsn_parameters()["host"] not in {"localhost", "127.0.0.1", "::1"}:
        raise ValueError("La certificación requiere un servidor local de pruebas")
    admin.autocommit = True
    database = "ca_cert_" + uuid4().hex
    parameters = admin.get_dsn_parameters()
    processes, handles = [], []
    server = thread = source_engine = async_engine = connection = cache = None
    created = False
    cache_owned = False
    tokens = {key: secrets.token_hex(32) for key in ("events", "source", "registry", "query", "resolver")}
    try:
        with admin.cursor() as cursor:
            cursor.execute(sql.SQL("CREATE DATABASE {}").format(sql.Identifier(database)))
        created = True
        connection = psycopg2.connect(admin_dsn, dbname=database)
        connection.autocommit = True

        def execute(statement, values=None):
            with connection.cursor() as cursor:
                cursor.execute(statement, values)
                return cursor.fetchall() if cursor.description else None

        migrations = sorted((root / "migrations/postgres").glob("*.up.sql"))
        for migration in migrations[:4]:
            execute(migration.read_text(encoding="utf-8-sig"))
        execute("INSERT INTO analytics_events(event_id,tenant_id,site_id,site_code,event_name,event_time,received_at) VALUES('legacy','tenant-a','1','site','page_view',now(),now())")
        for migration in migrations[4:]:
            execute(migration.read_text(encoding="utf-8-sig"))
        assert execute("SELECT event_source FROM analytics_events WHERE event_id='legacy'")[0][0] == "web"

        execute("CREATE SCHEMA source")
        # psycopg2 no expone siempre password en get_dsn_parameters.
        from sqlalchemy.engine import make_url
        source_url = make_url(admin_dsn.replace("postgres://", "postgresql://", 1)).set(drivername="postgresql+psycopg2", database=database)
        source_engine = create_engine(source_url, connect_args={"options": "-csearch_path=source"})
        with source_engine.begin() as local:
            local.execute(text("CREATE TABLE users(id BIGINT PRIMARY KEY,username VARCHAR(50),email VARCHAR(50),first_name VARCHAR(50),last_name VARCHAR(50),avatar_url VARCHAR(2048),hashed_password VARCHAR(128),created_at TIMESTAMPTZ DEFAULT now(),is_active BOOLEAN,type VARCHAR(20),note TEXT)"))
            local.execute(text("CREATE TABLE phones(id INTEGER PRIMARY KEY,number VARCHAR(20),user_id BIGINT)"))
            local.execute(text("INSERT INTO users(id,username,hashed_password,type,is_active) VALUES(42,'cliente-prueba','test','client',true)"))
            # Lee el SQL declarado para evitar el paquete Alembic legacy homónimo.
            import ast
            migration_path = dragon_root / "backend/df_av_back/alembic/versions_tenant/2026_11_13_0701-customer_analytics_lifecycle.py"
            module = ast.parse(migration_path.read_text(encoding="utf-8"))
            migration_sql = next(ast.literal_eval(node.value) for node in module.body if isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == "UPGRADE_SQL" for target in node.targets))
            local.exec_driver_sql(migration_sql)
            local.execute(text("SELECT analytics_customer_lifecycle_configure(true)"))

        site = SimpleNamespace(code="site", db_name="tenant-a", id=1, is_active=True, domains=["localhost"])
        source._load_sites = lambda: [site, SimpleNamespace(db_name="tenant-b")]
        source.get_sync_tenant_session_by_name = lambda _: Session(source_engine)
        routes = load_module("ca_cert_source_routes", dragon_root / "backend/df_av_back/api/v1/internal/analytics.py")
        routes._load_site_from_db = lambda _: site
        routes._load_site_timezone = lambda _: "UTC"
        routes._subject_snapshot_store = SubjectSnapshotStore(secret=tokens["source"], loader=source.load_customer_subjects)
        settings.SUBJECT_SNAPSHOT_ENABLED = settings.SUBJECT_CONTEXT_REGISTRY_ENABLED = True
        settings.SUBJECT_SOURCE_TOKEN = tokens["source"]
        settings.SUBJECT_CONTEXT_REGISTRY_TOKEN = tokens["registry"]
        settings.SITE_RESOLVER_TOKEN = tokens["resolver"]
        app = FastAPI()
        app.include_router(routes.router, prefix="/api/v1/internal")
        source_port, ingest_port, health_port, query_port = (free_port() for _ in range(4))
        source_base = f"http://127.0.0.1:{source_port}"
        server = uvicorn.Server(uvicorn.Config(app, host="127.0.0.1", port=source_port, log_level="error"))
        thread = threading.Thread(target=server.run, daemon=True)
        thread.start()
        wait_for(lambda: httpx.get(source_base + "/openapi.json").status_code == 200, "fuente HTTP")
        assert httpx.get(source_base + "/api/v1/internal/analytics/subjects/snapshot").status_code == 401

        cache = redis.Redis.from_url("redis://" + os.environ["CA_CERT_REDIS_ADDR"] + "/15")
        if cache.dbsize():
            raise ValueError("Redis DB 15 debe estar vacío y dedicado a esta certificación")
        cache_owned = True
        env = dict(os.environ, ENV_FILE="", REDIS_ADDR=os.environ["CA_CERT_REDIS_ADDR"], REDIS_DB="15",
                   POSTGRES_HOST=parameters["host"], POSTGRES_PORT=parameters["port"], POSTGRES_USER=parameters["user"],
                   POSTGRES_PASSWORD=source_url.password or "", POSTGRES_DB=database, POSTGRES_SSLMODE="disable",
                   GO_ANALYTICS_JWT_SECRET=secrets.token_hex(32), SUBJECT_INTERNAL_EVENTS_ENABLED="true", SUBJECT_ANALYTICS_ENABLED="true",
                   SUBJECT_BOOTSTRAP_ENABLED="true", SUBJECT_PROJECTION_ENABLED="true", SUBJECT_CONTEXT_REGISTRY_ENABLED="true", SUBJECT_QUERY_API_ENABLED="true",
                   SUBJECT_EVENTS_API_TOKEN=tokens["events"], SUBJECT_SOURCE_TOKEN=tokens["source"], SUBJECT_CONTEXT_REGISTRY_TOKEN=tokens["registry"], SUBJECT_QUERY_API_TOKEN=tokens["query"],
                   SUBJECT_SNAPSHOT_URL=source_base + "/api/v1/internal/analytics/subjects/snapshot", SUBJECT_CONTEXT_REGISTRY_URL=source_base + "/api/v1/internal/analytics/context-registry",
                   SITE_RESOLVER_URL=source_base + "/api/v1/internal/analytics/sites/resolve", SITE_RESOLVER_TOKEN=tokens["resolver"], SITE_RESOLVER_TIMEOUT_MS="5000",
                   INGEST_HTTP_HOST="127.0.0.1", INGEST_HTTP_PORT=str(ingest_port), WORKER_HEALTH_HTTP_HOST="127.0.0.1", WORKER_HEALTH_HTTP_PORT=str(health_port),
                   SUBJECT_QUERY_HTTP_HOST="127.0.0.1", SUBJECT_QUERY_HTTP_PORT=str(query_port), RATE_LIMIT_ENABLED="false", EVENT_STREAM_NAME="ca-cert:"+database)

        with tempfile.TemporaryDirectory(prefix="ca-cert-") as temporary:
            folder = Path(temporary)
            binaries = {}
            for name in ("ingest", "worker", "subject-jobs", "query-api"):
                target = folder / (name + (".exe" if os.name == "nt" else ""))
                module_path = root / "services" / ("ingest" if name == "ingest" else "worker")
                subprocess.run(["go", "build", "-o", str(target), "./cmd/"+name], cwd=module_path, check=True)
                binaries[name] = target

            def start(name, overrides=None):
                handle = (folder / (name + ".log")).open("a", encoding="utf-8")
                handles.append(handle)
                process = subprocess.Popen([str(binaries[name])], cwd=folder, env=dict(env, **(overrides or {})), stdout=handle, stderr=handle)
                processes.append(process)
                return process

            def job(mode, *arguments):
                result = subprocess.run([str(binaries["subject-jobs"]), "--mode", mode, *arguments], cwd=folder, env=env, capture_output=True, text=True, timeout=60)
                assert result.returncode == 0, result.stderr

            try:
                ingest_process = start("ingest")
                worker_process = start("worker")
                query_process = start("query-api")
                ingest_url = f"http://127.0.0.1:{ingest_port}/v1/internal/events"
                query_url = f"http://127.0.0.1:{query_port}/v1/internal/subjects/suggestions"
                wait_for(lambda: httpx.get(f"http://127.0.0.1:{health_port}/ready").status_code == 200, "worker")
                wait_for(lambda: httpx.get(query_url).status_code == 401, "query API")
                job("refresh-registry")
                job("bootstrap")
                job("bootstrap")
                assert execute("SELECT count(*) FROM analytics_subject_bootstrap")[0][0] == 1
                assert execute("SELECT count(*) FROM analytics_subjects")[0][0] == 2
                emitter = subject_events.SubjectEventEmitter(enabled=True, url=ingest_url, token=tokens["events"])
                event = subject_events.association_event(site_code="site", customer_id=42, operation_type="appointment", operation_id=8, relation="attendee", occurred_at=datetime.now(timezone.utc), context_key="appointments.customer_selection", context_version=1, claimed_context_family="selection")
                assert asyncio.run(emitter.emit(event)).outcome == "accepted"
                wait_for(lambda: execute("SELECT count(*) FROM analytics_event_subjects WHERE normalization_status='normalized' AND context_resolution_status='registered'")[0][0] == 1, "asociación normalizada")
                assert asyncio.run(emitter.emit(event)).outcome == "accepted"
                job("resolve-initial-contexts")
                job("project-global")
                job("project-context")
                headers = {"Authorization": "Bearer " + tokens["query"]}
                params = dict(tenant_id="tenant-a", subject_type="customer", context_key="appointments.customer_selection", context_version=1, limit=10)
                response = httpx.get(query_url, params=params, headers=headers)
                assert response.status_code == 200, response.text
                payload = response.json()
                assert payload["results"][0]["external_subject_id"] == "42"
                assert payload["context"]["source_level"] == "context"
                assert payload["results"][0]["association_count"] == 1
                assert httpx.get(query_url, params=dict(params, tenant_id="tenant-b"), headers=headers).json()["results"] == []
                assert not any(key in response.text for key in ("cliente-prueba", "email", "analytics_subject_id"))
                with source_engine.begin() as local:
                    local.execute(text("INSERT INTO users(id,username,hashed_password,type,is_active) VALUES(43,'otro-cliente','test','client',true)"))
                global_event = subject_events.association_event(site_code="site",customer_id=43,operation_type="appointment",operation_id=9,relation="attendee",occurred_at=datetime.now(timezone.utc))
                assert asyncio.run(emitter.emit(global_event)).outcome == "accepted"
                wait_for(lambda: execute("SELECT count(*) FROM analytics_event_subjects WHERE normalization_status='normalized'")[0][0] == 2,"asociación global")
                job("project-global")
                job("project-context")
                mixed = httpx.get(query_url,params=params,headers=headers).json()
                assert [value["external_subject_id"] for value in mixed["results"]] == ["42","43"]
                assert mixed["context"]["source_level"] == "mixed"
                worker_metrics = httpx.get(f"http://127.0.0.1:{health_port}/metrics").text
                query_metrics = httpx.get(f"http://127.0.0.1:{query_port}/metrics").text
                assert 'subject_normalization_total{' in worker_metrics
                assert 'subject_context_registry_loads_total{' in worker_metrics
                assert 'subject_query_requests_total{' in query_metrics
                assert 'subject_query_duration_seconds_count{' in query_metrics

                settings.SUBJECT_ANALYTICS_SUGGESTIONS_ENABLED = True
                settings.SUBJECT_ANALYTICS_SUGGESTION_TENANTS = ["tenant-a"]
                settings.GO_ANALYTICS_SUGGESTIONS_URL = query_url
                settings.SUBJECT_QUERY_API_TOKEN = tokens["query"]
                subject_suggestions.get_current_db_name = lambda: "tenant-a"
                async_engine = create_async_engine(source_url.set(drivername="postgresql+asyncpg"), connect_args={"server_settings": {"search_path": "source"}})

                async def hydrate():
                    async with AsyncSession(async_engine) as session:
                        values = await subject_suggestions.suggested_customers(session, limit=10, context_key="appointments.customer_selection", context_version=1)
                        assert values and values[0].id == 42
                        assert values[0].username == "cliente-prueba"
                    await async_engine.dispose()
                asyncio.run(hydrate())

                with source_engine.begin() as local:
                    local.execute(text("UPDATE users SET is_active=false WHERE id=42"))
                settings.SUBJECT_ANALYTICS_EMISSION_ENABLED = True
                settings.GO_ANALYTICS_INTERNAL_EVENTS_URL = ingest_url
                settings.SUBJECT_EVENTS_API_TOKEN = tokens["events"]
                subject_events.get_current_site_code = lambda: "site"
                subject_events.get_current_db_name = lambda: "tenant-a"
                assert asyncio.run(subject_events.emit_customer_deactivated(SimpleNamespace(id=42))).outcome == "accepted"
                wait_for(lambda: execute("SELECT lifecycle_status FROM analytics_subjects WHERE tenant_id='tenant-a' AND external_subject_id='42'")[0][0] == "inactive", "lifecycle post-commit")
                assert [value["external_subject_id"] for value in httpx.get(query_url, params=params, headers=headers).json()["results"]] == ["43"]
                job("project-global")
                job("project-context")
                with source_engine.begin() as local:
                    local.execute(text("UPDATE users SET is_active=true WHERE id=42"))
                row = source.load_customer_lifecycle("tenant-a",42)
                reactivation = subject_events.lifecycle_event(site_code="site",customer_id=42,event_name="subject_reactivated",source_updated_at=datetime.fromisoformat(row.source_updated_at.replace("Z","+00:00")),source_mutation_id=row.source_mutation_id)
                assert asyncio.run(emitter.emit(reactivation)).outcome == "accepted"
                wait_for(lambda: execute("SELECT lifecycle_status FROM analytics_subjects WHERE tenant_id='tenant-a' AND external_subject_id='42'")[0][0] == "active","reactivación")
                restored = httpx.get(query_url,params=params,headers=headers).json()["results"]
                assert restored[0]["external_subject_id"] == "42" and restored[0]["association_count"] == 1
                settings.SUBJECT_ANALYTICS_SUGGESTIONS_ENABLED = False
                assert asyncio.run(subject_suggestions.suggested_customers(None, limit=10)) is None
                assert execute("SELECT count(*) FROM analytics_events WHERE event_source='server'")[0][0] == 4
                for process in (ingest_process,worker_process,query_process):
                    process.terminate()
                    process.wait(timeout=10)
                disabled = {key: "false" for key in ("SUBJECT_INTERNAL_EVENTS_ENABLED", "SUBJECT_ANALYTICS_ENABLED", "SUBJECT_BOOTSTRAP_ENABLED", "SUBJECT_PROJECTION_ENABLED", "SUBJECT_CONTEXT_REGISTRY_ENABLED", "SUBJECT_QUERY_API_ENABLED")}
                start("ingest", disabled)
                start("worker", disabled)
                wait_for(lambda: httpx.get(f"http://127.0.0.1:{ingest_port}/ready").status_code == 200, "ingest legacy")
                assert httpx.post(ingest_url, json=event, headers={"Authorization":"Bearer "+tokens["events"]}).status_code == 404
                import jwt
                now = int(time.time())
                tracking = jwt.encode(dict(iss="main-backend",aud="analytics-ingest",site_code="site",env="production",token_version=1,jti="rollback",iat=now,nbf=now-1,exp=now+600),env["GO_ANALYTICS_JWT_SECRET"],algorithm="HS256")
                web = dict(event_id="web-after-rollback",event_name="page_view",event_version=1,timestamp=datetime.now(timezone.utc).isoformat(),anonymous_id="anonymous-test",session_id="session-test",origin="http://localhost",url="http://localhost/",path="/",properties={},context={})
                response = httpx.post(f"http://127.0.0.1:{ingest_port}/v1/events",json=web,headers={"Authorization":"Bearer "+tracking,"Origin":"http://localhost"})
                assert response.status_code == 202, response.text
                wait_for(lambda: execute("SELECT count(*) FROM analytics_events WHERE event_id='web-after-rollback' AND event_source='web'")[0][0] == 1, "persistencia web con flags apagados")
                assert execute("SELECT count(*) FROM analytics_event_subjects")[0][0] == 4
            except Exception:
                for handle in handles:
                    handle.flush()
                for path in folder.glob("*.log"):
                    print(path.name + ":\n" + path.read_text(encoding="utf-8", errors="replace")[-5000:], file=sys.stderr)
                raise
            finally:
                for process in processes:
                    if process.poll() is None:
                        process.terminate()
                for process in processes:
                    process.wait(timeout=10)
                for handle in handles:
                    handle.close()
        for migration in reversed(migrations[4:]):
            execute(migration.with_name(migration.name.replace(".up.sql", ".down.sql")).read_text(encoding="utf-8-sig"))
        assert execute("SELECT count(*) FROM analytics_events WHERE event_id='legacy'")[0][0] == 1
        for migration in reversed(migrations[:4]):
            execute(migration.with_name(migration.name.replace(".up.sql", ".down.sql")).read_text(encoding="utf-8-sig"))
        assert execute("SELECT to_regclass('analytics_events')")[0][0] is None
        print(json.dumps({"result": "passed", "steps": ["migrations_004_to_008_and_down", "source_snapshot", "bootstrap_noop", "dedicated_bearer", "real_emitter", "redis", "resolver_positive_version", "worker_normalization", "registry_cache_ids", "global_context_projection", "query", "tenant_isolation", "real_sql_hydration", "postcommit_lifecycle", "all_subject_flags_off_with_web_persistence"]}))
    finally:
        if server:
            server.should_exit = True
        if thread:
            thread.join(timeout=10)
        if source_engine:
            source_engine.dispose()
        if connection:
            connection.close()
        if cache:
            if cache_owned:
                cache.flushdb()
            cache.close()
        if created:
            with admin.cursor() as cursor:
                cursor.execute(sql.SQL("DROP DATABASE {} WITH (FORCE)").format(sql.Identifier(database)))
        admin.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dragon-root", type=Path, required=True)
    certify(parser.parse_args().dragon_root.resolve())
