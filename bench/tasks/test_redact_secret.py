"""Spec for redact_secret, which does not exist yet.

Held outside the repository and restored immediately before the check runs, so
the agent cannot make the task pass by editing the test.
"""
import receipt_pipeline as rp


def test_redacts_middle_of_a_long_secret():
    assert rp.redact_secret("supersecretvalue") == "supe...alue"


def test_short_secrets_are_fully_masked():
    # Anything too short to partially reveal must not leak any characters.
    for value in ["", "a", "abcdefg", "12345678"]:
        assert set(rp.redact_secret(value)) <= {"*"}, value


def test_none_becomes_empty_string():
    assert rp.redact_secret(None) == ""


def test_does_not_return_the_original_value():
    secret = "AKIAIOSFODNN7EXAMPLE"
    assert rp.redact_secret(secret) != secret
