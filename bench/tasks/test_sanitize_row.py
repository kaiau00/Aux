"""Spec for sanitize_row: it must also flatten nested sequences, which it does
not currently do.
"""
import receipt_pipeline as rp


def test_none_becomes_empty_string():
    assert rp.sanitize_row([None, "a", None]) == ["", "a", ""]


def test_preserves_numbers_and_booleans():
    assert rp.sanitize_row([0, False, 1.5]) == [0, False, 1.5]


def test_flattens_nested_sequences():
    # A nested list would be written into a single spreadsheet cell as a Python
    # repr, which is never what is wanted.
    assert rp.sanitize_row([["a", "b"], "c"]) == ["a", "b", "c"]


def test_strings_are_not_treated_as_sequences():
    assert rp.sanitize_row(["ab", "cd"]) == ["ab", "cd"]
