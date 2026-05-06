# Guia para agente IA del backend principal

## Objetivo

Implementar la integracion del backend principal con Go Analytics sin acoplar
el proyecto principal al codigo interno del microservicio.

El backend principal debe cumplir contratos publicos y operativos:

- Generar JWT de tracking compatible con `docs/jwt-contract.md`.
- Entregar el tracking token al frontend para inicializar el SDK.
- Hidratar Redis con metadata de site.
- Exponer un resolver interno compatible con `docs/resolver-contract.md`.

Go Analytics debe seguir consumiendo solo JWT, metadata Redis y una URL interna
configurable. No debe importar modelos, servicios ni librerias del backend
principal.

## Regla principal de dependencias

Las dependencias deben apuntar hacia adentro en cada proyecto.

En el backend principal, la logica de dominio propia no debe depender de Redis,
HTTP externo ni detalles concretos de Go Analytics. La integracion debe quedar
en servicios de aplicacion o adaptadores de infraestructura del backend
principal.

En Go Analytics, se mantiene la regla:

```text
adapters -> application -> domain
```

No modificar `domain` ni `application` de Go Analytics para conocer FastAPI,
Django, Node.js, tablas tenant, modelos ORM del backend principal ni nombres
internos del proyecto integrador.

## Tareas del agente IA

1. Generar JWT de tracking.

   El token debe estar firmado con el algoritmo configurado para Go Analytics.
   En la version actual se usa HS256. Los claims obligatorios deben coincidir
   con `docs/jwt-contract.md`: `iss`, `aud`, `site_code`, `env`,
   `token_version`, `iat`, `nbf`, `exp` y `jti`.

   No incluir secretos, credenciales, datos personales ni IDs internos como
   campos principales del token. Usar `site_code` como identificador publico.

2. Entregar token al frontend.

   Crear o adaptar un endpoint del backend principal para devolver el tracking
   token al frontend autenticado o autorizado segun las reglas del proyecto
   principal.

   El frontend debe inicializar el SDK con ese token y con el endpoint publico
   de ingesta de Go Analytics. El SDK no debe firmar tokens ni conocer secretos.

3. Hidratar Redis.

   Crear una funcion del backend principal que escriba metadata de site en la
   clave:

   ```text
   goanalytics:site:public:{site_code}
   ```

   El valor JSON debe incluir como minimo:

   ```json
   {
     "site_code": "pub_site_abc123",
     "tenant_id": "tenant_123",
     "site_id": "site_456",
     "status": "active",
     "tracking_enabled": true,
     "allowed_domains": ["cliente.com", "www.cliente.com"],
     "token_version": 1,
     "sample_rate": 1,
     "schema_version": 1
   }
   ```

   El TTL debe ser compatible con la configuracion `SITE_CACHE_TTL_SECONDS` del
   worker. Si el backend principal usa otro nombre interno para tenant o site,
   mapearlo solo en esta capa de integracion.

4. Implementar resolver interno.

   Exponer un endpoint interno compatible con `docs/resolver-contract.md`:

   ```http
   POST /internal/analytics/sites/resolve
   Authorization: Bearer <SITE_RESOLVER_TOKEN>
   Content-Type: application/json
   ```

   El request esperado es:

   ```json
   {
     "site_code": "pub_site_abc123",
     "origin": "https://cliente.com",
     "env": "production"
   }
   ```

   Si el site existe, devolver la metadata normalizada. Si no existe, responder
   404 con `{"error":"site_not_found"}`. No devolver stack traces ni detalles
   internos del backend principal.

5. Validar rehidratacion automatica.

   Probar que, si Redis no tiene `goanalytics:site:public:{site_code}`, el
   worker llama al resolver interno, recibe metadata valida, actualiza cache y
   procesa eventos posteriores.

## Criterios de aceptacion

- El backend principal genera tokens que Go Ingest acepta.
- El frontend recibe token sin exponer secretos.
- Redis contiene metadata de site con el contrato esperado.
- El worker puede rehidratar metadata desde `SITE_RESOLVER_URL`.
- El resolver interno valida `SITE_RESOLVER_TOKEN`.
- No se modifican contratos publicos de Go Analytics sin actualizar `docs/*`.
- No se agregan imports de infraestructura concreta en `domain` o
  `application` de Go Analytics.

## Validaciones recomendadas

- Prueba unitaria para generacion de claims JWT.
- Prueba de contrato del endpoint de token para frontend.
- Prueba de serializacion de metadata Redis.
- Prueba de contrato del resolver interno con site existente y no existente.
- Prueba manual de rehidratacion con Redis limpio.
