import pytest
import re


class TestValidateIdentifier:
    def test_valid_database_name(self):
        from security.sql_utils import validate_identifier
        assert validate_identifier('mydb') == 'mydb'

    def test_valid_table_name(self):
        from security.sql_utils import validate_identifier
        assert validate_identifier('users') == 'users'

    def test_valid_name_with_underscore(self):
        from security.sql_utils import validate_identifier
        assert validate_identifier('my_table') == 'my_table'

    def test_valid_name_with_digits(self):
        from security.sql_utils import validate_identifier
        assert validate_identifier('table123') == 'table123'

    def test_valid_name_start_with_underscore(self):
        from security.sql_utils import validate_identifier
        assert validate_identifier('_private') == '_private'

    def test_reject_empty_string(self):
        from security.sql_utils import validate_identifier
        with pytest.raises(ValueError, match='empty'):
            validate_identifier('')

    def test_reject_semicolon_injection(self):
        from security.sql_utils import validate_identifier
        with pytest.raises(ValueError, match='invalid'):
            validate_identifier('db`; DROP TABLE users; --')

    def test_reject_quote_injection(self):
        from security.sql_utils import validate_identifier
        with pytest.raises(ValueError, match='invalid'):
            validate_identifier("db'; DROP TABLE users; --")

    def test_reject_double_dash_comment(self):
        from security.sql_utils import validate_identifier
        with pytest.raises(ValueError, match='invalid'):
            validate_identifier('db--comment')

    def test_reject_space_in_name(self):
        from security.sql_utils import validate_identifier
        with pytest.raises(ValueError, match='invalid'):
            validate_identifier('my db')

    def test_reject_over_64_chars(self):
        from security.sql_utils import validate_identifier
        with pytest.raises(ValueError, match='too long'):
            validate_identifier('a' * 65)

    def test_max_64_chars_valid(self):
        from security.sql_utils import validate_identifier
        assert validate_identifier('a' * 64) == 'a' * 64


class TestEscapeIdentifier:
    def test_simple_name(self):
        from security.sql_utils import escape_identifier
        assert escape_identifier('mydb') == '`mydb`'

    def test_name_with_backtick(self):
        from security.sql_utils import escape_identifier
        with pytest.raises(ValueError):
            escape_identifier('my`db')

    def test_reserved_word_escaped(self):
        from security.sql_utils import escape_identifier
        assert escape_identifier('select') == '`select`'


class TestBuildQualifiedTableName:
    def test_normal_name(self):
        from security.sql_utils import build_qualified_name
        assert build_qualified_name('mydb', 'mytable') == '`mydb`.`mytable`'

    def test_injection_rejected(self):
        from security.sql_utils import build_qualified_name
        with pytest.raises(ValueError):
            build_qualified_name('db`; DROP TABLE t; --', 'mytable')

    def test_table_injection_rejected(self):
        from security.sql_utils import build_qualified_name
        with pytest.raises(ValueError):
            build_qualified_name('mydb', 't`; DROP TABLE t; --')


class TestSafeShowCreateTable:
    def test_generates_safe_sql(self):
        from security.sql_utils import safe_show_create_table
        sql = safe_show_create_table('mydb', 'mytable')
        assert sql == 'SHOW CREATE TABLE `mydb`.`mytable`'

    def test_reject_malicious_db(self):
        from security.sql_utils import safe_show_create_table
        with pytest.raises(ValueError):
            safe_show_create_table('db`; DROP TABLE t; --', 'mytable')


class TestSafeSelectCount:
    def test_generates_safe_sql(self):
        from security.sql_utils import safe_select_count
        sql = safe_select_count('mydb', 'mytable')
        assert sql == 'SELECT COUNT(*) FROM `mydb`.`mytable`'


class TestSafeSelectBatch:
    def test_generates_safe_sql(self):
        from security.sql_utils import safe_select_batch
        sql = safe_select_batch('mydb', 'mytable', limit=1000, offset=0)
        assert sql == 'SELECT * FROM `mydb`.`mytable` LIMIT 1000 OFFSET 0'

    def test_negative_limit_rejected(self):
        from security.sql_utils import safe_select_batch
        with pytest.raises(ValueError):
            safe_select_batch('mydb', 'mytable', limit=-1, offset=0)

    def test_negative_offset_rejected(self):
        from security.sql_utils import safe_select_batch
        with pytest.raises(ValueError):
            safe_select_batch('mydb', 'mytable', limit=1000, offset=-1)

    def test_non_integer_rejected(self):
        from security.sql_utils import safe_select_batch
        with pytest.raises((ValueError, TypeError)):
            safe_select_batch('mydb', 'mytable', limit='drop', offset=0)


class TestSafeInsertSQL:
    def test_generates_safe_sql(self):
        from security.sql_utils import safe_insert_sql
        cols = ['id', 'name', 'age']
        sql = safe_insert_sql('mydb', 'mytable', cols)
        assert '`mydb`.`mytable`' in sql
        assert '`id`' in sql
        assert '`name`' in sql
        assert '`age`' in sql
        assert '%s' in sql
        assert 'INSERT INTO' in sql

    def test_column_injection_rejected(self):
        from security.sql_utils import safe_insert_sql
        with pytest.raises(ValueError):
            safe_insert_sql('mydb', 'mytable', ['id`, col2'])


class TestSafeShowTableStatus:
    def test_generates_safe_sql(self):
        from security.sql_utils import safe_show_table_status
        sql = safe_show_table_status('mydb')
        assert sql == 'SHOW TABLE STATUS FROM `mydb`'

    def test_injection_rejected(self):
        from security.sql_utils import safe_show_table_status
        with pytest.raises(ValueError):
            safe_show_table_status('db`; DROP TABLE t; --')


class TestSafeTableStatistics:
    def test_generates_parameterized_sql(self):
        from security.sql_utils import safe_table_statistics
        sql, params = safe_table_statistics('mydb')
        assert '%s' in sql
        assert params == ('mydb',)
        assert 'information_schema.tables' in sql

    def test_validates_identifier(self):
        from security.sql_utils import safe_table_statistics
        with pytest.raises(ValueError):
            safe_table_statistics('db`; DROP TABLE t; --')


class TestSafeDropTable:
    def test_generates_safe_sql(self):
        from security.sql_utils import safe_drop_table_if_exists
        sql = safe_drop_table_if_exists('mydb', 'mytable')
        assert sql == 'DROP TABLE IF EXISTS `mydb`.`mytable`'

    def test_injection_rejected(self):
        from security.sql_utils import safe_drop_table_if_exists
        with pytest.raises(ValueError):
            safe_drop_table_if_exists('mydb', 't`; DROP TABLE mysql.user; --')
