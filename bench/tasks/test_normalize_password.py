"""Spec for normalize_gmail_app_password.

Most of this already passes; the gap is that None raises TypeError instead of
returning an empty string. Existing behaviour is pinned so a fix cannot quietly
break it.
"""
import receipt_pipeline as rp


def test_strips_regular_spaces():
    assert rp.normalize_gmail_app_password("abcd efgh ijkl mnop") == "abcdefghijklmnop"


def test_strips_non_breaking_spaces():
    # Google's UI copies NBSP (U+00A0) between groups. str.isspace() already
    # covers this; pinned so a rewrite does not lose it.
    assert rp.normalize_gmail_app_password("abcd efgh") == "abcdefgh"


def test_strips_tabs_and_newlines():
    assert rp.normalize_gmail_app_password("ab\tcd\nef") == "abcdef"


def test_empty_and_none_safe():
    assert rp.normalize_gmail_app_password("") == ""
    assert rp.normalize_gmail_app_password(None) == ""
