"""Pruebas del script en su imagen real, sin conectarse a bases de usuario."""

import os
import subprocess
import unittest


@unittest.skipUnless(os.environ.get("RUN_MIGRATE_CONTAINER_TESTS") == "1", "Requiere Docker e imagen construida")
class MigrationScriptTests(unittest.TestCase):
    """Comprueba argumentos, validación, códigos de salida y secretos."""

    def run_script(self, overrides=None, result="0"):
        """Ejecuta el script con un doble de migrate dentro del contenedor."""
        values = {
            "POSTGRES_HOST": "postgres", "POSTGRES_USER": "a@b",
            "POSTGRES_PASSWORD": "p:/?#% $'á", "POSTGRES_DB": "db/name",
            "EXPECTED_URL": "postgres://%61%40%62:%70%3a%2f%3f%23%25%20%24%27%c3%a1@postgres:5432/%64%62%2f%6e%61%6d%65?sslmode=disable",
            "TEST_EXIT": result,
        }
        values.update(overrides or {})
        command = ["docker", "run", "--rm", "--entrypoint", "/bin/sh"]
        for key, value in values.items():
            command.extend(["-e", f"{key}={value}"])
        command.extend([
            os.environ.get("MIGRATE_TEST_IMAGE", "goanalytics-migrate:test"), "-c",
            'migrate() { [ "$1" = "-path=/migrations" ] && [ "$2" = "-database=$EXPECTED_URL" ] && [ "$3" = up ] || return 99; echo "$POSTGRES_PASSWORD" >&2; return "$TEST_EXIT"; }; . /usr/local/bin/migrate-up.sh',
        ])
        return subprocess.run(command, capture_output=True, text=True, encoding="utf-8")

    def test_codificacion_y_sin_secretos(self):
        result = self.run_script()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("actualizada", result.stdout)
        self.assertNotIn("p:/?#%", result.stdout + result.stderr)

    def test_fallo_conserva_codigo_y_oculta_salida(self):
        result = self.run_script(result="7")
        self.assertEqual(result.returncode, 7)
        self.assertNotIn("p:/?#%", result.stdout + result.stderr)

    def test_configuracion_invalida(self):
        for values in ({"POSTGRES_PASSWORD": ""}, {"POSTGRES_PORT": "abc"}, {"POSTGRES_PORT": "65536"}, {"POSTGRES_SSLMODE": "invalid"}):
            with self.subTest(values=values):
                self.assertEqual(self.run_script(values).returncode, 2)


if __name__ == "__main__":
    unittest.main()
