import pytest
import os

_BASE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def _read_source(rel_path):
    with open(os.path.join(_BASE, rel_path), 'r', encoding='utf-8') as f:
        return f.read()


class TestSyncEngineIdentifierValidation:
    def test_sync_engine_imports_validate(self):
        source = _read_source('instances/sync_engine.py')
        assert 'validate_identifier' in source
        assert 'safe_show_create_table' in source
        assert 'safe_select_count' in source
        assert 'safe_insert_sql' in source

    def test_sync_engine_no_fstring_sql(self):
        source = _read_source('instances/sync_engine.py')
        dangerous = ['f"SELECT', 'f"SHOW', 'f"DROP', 'f"INSERT']
        for p in dangerous:
            assert p not in source, f"发现危险SQL拼接: {p}"


class TestViewsIdentifierValidation:
    def test_views_imports_safe_sql(self):
        source = _read_source('instances/views.py')
        assert 'safe_show_table_status' in source
        assert 'validate_identifier' in source


class TestCollectorParameterizedQuery:
    def test_collector_uses_parameterized_query(self):
        source = _read_source('monitor/mysql_collector.py')
        assert 'safe_table_statistics' in source
        assert 'validate_identifier' in source


class TestAuditLogCleanupPerformance:
    def test_save_does_not_do_cleanup(self):
        source = _read_source('security/models.py')
        audit_section = source.split('class AuditLog')[1].split('class PasswordPolicy')[0]
        assert 'AuditLog.objects.count()' not in audit_section
        assert 'AuditLog.objects.order_by' not in audit_section

    def test_cleanup_task_exists(self):
        source = _read_source('monitor/tasks.py')
        assert 'cleanup_old_audit_logs' in source


class TestAlertDeduplication:
    def test_evaluate_rule_updates_existing_alert(self):
        source = _read_source('monitor/tasks.py')
        assert "status='firing'" in source
        assert 'existing' in source
        assert 'resolved' in source


class TestSSHUpgradeSecurity:
    def test_no_skip_grant_tables(self):
        source = _read_source('instances/upgrade_engine.py')
        assert '--skip-grant-tables' not in source

    def test_no_auto_add_policy(self):
        source = _read_source('instances/upgrade_engine.py')
        assert 'AutoAddPolicy' not in source
        assert 'RejectPolicy' in source

    def test_password_uses_defaults_file(self):
        source = _read_source('instances/upgrade_engine.py')
        assert '--defaults-extra-file' in source
        assert "-p'" not in source

    def test_loads_known_hosts(self):
        source = _read_source('instances/upgrade_engine.py')
        assert 'load_host_keys' in source


class TestMySQLConnectionConfig:
    def test_collector_has_retries(self):
        source = _read_source('monitor/mysql_collector.py')
        assert 'max_retries' in source

    def test_collector_has_read_timeout(self):
        source = _read_source('monitor/mysql_collector.py')
        assert 'read_timeout' in source
        assert 'write_timeout' in source

    def test_collector_decrypts_password(self):
        source = _read_source('monitor/mysql_collector.py')
        assert 'decrypt_password()' in source


class TestSettingsSecurity:
    def test_settings_has_conn_max_age(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'CONN_MAX_AGE' in source
        assert 'CONN_HEALTH_CHECKS' in source

    def test_settings_has_mysql_as_default(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'django.db.backends.mysql' in source

    def test_settings_has_charset_utf8mb4(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'utf8mb4' in source

    def test_settings_secret_key_warns_default(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'warnings.warn' in source


class TestCeleryBeatSchedule:
    def test_audit_cleanup_in_schedule(self):
        source = _read_source('mysql_dba_platform/settings.py')
        assert 'cleanup-old-audit-logs-daily' in source


class TestPermissionModule:
    def test_has_readonly_permission(self):
        source = _read_source('security/permissions.py')
        assert 'IsReadOnly' in source
        assert "'readonly'" in source
