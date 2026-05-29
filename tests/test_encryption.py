import pytest
import base64
import hashlib
from unittest.mock import patch, MagicMock


def _get_fernet_key():
    from cryptography.fernet import Fernet
    secret_key = 'test-secret-key-for-unit-testing-32b!'
    key_material = secret_key.encode()
    hashed = hashlib.sha256(key_material).digest()
    return base64.urlsafe_b64encode(hashed)


class TestFernetKeyGeneration:
    def test_key_is_valid_fernet_key(self):
        from cryptography.fernet import Fernet
        key = _get_fernet_key()
        assert len(key) == 44
        Fernet(key)

    def test_key_is_deterministic(self):
        key1 = _get_fernet_key()
        key2 = _get_fernet_key()
        assert key1 == key2

    def test_key_is_32_bytes_base64(self):
        key = _get_fernet_key()
        decoded = base64.urlsafe_b64decode(key)
        assert len(decoded) == 32


class TestEncryptionDecryption:
    def test_encrypt_decrypt_roundtrip(self):
        from cryptography.fernet import Fernet
        key = _get_fernet_key()
        f = Fernet(key)
        raw = 'MySecretPassword123!'
        encrypted = f.encrypt(raw.encode()).decode()
        assert encrypted != raw
        decrypted = f.decrypt(encrypted.encode()).decode()
        assert decrypted == raw

    def test_encrypt_not_return_plaintext(self):
        from cryptography.fernet import Fernet
        key = _get_fernet_key()
        f = Fernet(key)
        raw = 'TestPassword456'
        encrypted = f.encrypt(raw.encode()).decode()
        assert encrypted != raw

    def test_different_passwords_different_ciphertext(self):
        from cryptography.fernet import Fernet
        key = _get_fernet_key()
        f = Fernet(key)
        enc1 = f.encrypt(b'password1').decode()
        enc2 = f.encrypt(b'password2').decode()
        assert enc1 != enc2

    def test_empty_password_rejected(self):
        with pytest.raises(ValueError):
            _validate_and_encrypt('')

    def test_decrypt_corrupted_ciphertext_raises(self):
        from cryptography.fernet import Fernet, InvalidToken
        key = _get_fernet_key()
        f = Fernet(key)
        with pytest.raises(InvalidToken):
            f.decrypt(b'not-valid-fernet-ciphertext')

    def test_sha256_key_derivation_not_truncation(self):
        secret_key = 'short-key'
        key_material = secret_key.encode()
        hashed = hashlib.sha256(key_material).digest()
        assert len(hashed) == 32
        fernet_key = base64.urlsafe_b64encode(hashed)
        assert len(fernet_key) == 44


def _validate_and_encrypt(raw_password):
    if not raw_password:
        raise ValueError('password cannot be empty')
    from cryptography.fernet import Fernet
    key = _get_fernet_key()
    f = Fernet(key)
    return f.encrypt(raw_password.encode()).decode()


class TestDecryptUsedInCollector:
    def test_collect_instance_calls_decrypt_password(self):
        mock_instance = MagicMock()
        mock_instance.host = '192.168.1.100'
        mock_instance.port = 3306
        mock_instance.username = 'root'
        mock_instance.decrypt_password.return_value = 'decrypted_pass'

        with patch('monitor.mysql_collector.MySQLCollector') as MockCollector:
            MockCollector.return_value.collect_all.return_value = {'timestamp': '2026-01-01'}
            from monitor.mysql_collector import collect_instance_metrics
            collect_instance_metrics(mock_instance)
            mock_instance.decrypt_password.assert_called_once()
            call_args = MockCollector.call_args
            assert call_args[1].get('password') == 'decrypted_pass' or call_args[0][3] == 'decrypted_pass'
