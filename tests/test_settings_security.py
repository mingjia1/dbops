import pytest
import os


_BASE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def _read_source(rel_path):
    with open(os.path.join(_BASE, rel_path), 'r', encoding='utf-8') as f:
        return f.read()


class TestDatabaseBackendConfig:
    def test_mysql_as_default_engine(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'django.db.backends.mysql' in source

    def test_conn_max_age_configured(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'CONN_MAX_AGE' in source

    def test_conn_health_checks(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'CONN_HEALTH_CHECKS' in source

    def test_charset_utf8mb4(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'utf8mb4' in source

    def test_sql_mode_strict(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'STRICT_TRANS_TABLES' in source


class TestSecuritySettingsConfig:
    def test_secret_key_warns_on_default(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'DJANGO_SECRET_KEY' in source
        assert 'warnings.warn' in source
        assert 'RuntimeWarning' in source

    def test_debug_from_env(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'DJANGO_DEBUG' in source

    def test_allowed_hosts_from_env(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'DJANGO_ALLOWED_HOSTS' in source
