FROM migrate/migrate:v4.17.1

# El contexto de construcción es la raíz del repositorio.
COPY deploy/migrations/migrate-up.sh /usr/local/bin/migrate-up.sh
ENTRYPOINT ["/bin/sh", "/usr/local/bin/migrate-up.sh"]
CMD []
