"""Spec for parse_currency, which does not exist yet.

Receipt totals arrive as strings in several shapes; this is the parsing all of
them have to funnel through.
"""
import pytest

import receipt_pipeline as rp


@pytest.mark.parametrize(
    "raw,expected",
    [
        ("12.50", 12.50),
        ("$12.50", 12.50),
        ("$1,234.56", 1234.56),
        ("  $99  ", 99.0),
        ("0", 0.0),
        ("$0.00", 0.0),
    ],
)
def test_parses_common_shapes(raw, expected):
    assert rp.parse_currency(raw) == pytest.approx(expected)


@pytest.mark.parametrize("raw,expected", [("(12.00)", -12.00), ("-$5.25", -5.25)])
def test_parenthesised_and_signed_amounts_are_negative(raw, expected):
    # Accounting exports write negatives in parentheses.
    assert rp.parse_currency(raw) == pytest.approx(expected)


def test_numbers_pass_through():
    assert rp.parse_currency(7) == pytest.approx(7.0)
    assert rp.parse_currency(7.5) == pytest.approx(7.5)


@pytest.mark.parametrize("raw", [None, "", "   ", "n/a", "abc"])
def test_unparseable_values_return_none(raw):
    # Returning 0.0 for junk would silently corrupt a total.
    assert rp.parse_currency(raw) is None
