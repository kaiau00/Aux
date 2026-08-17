"""Spec for chunk_rows, which does not exist yet.

The Sheets API rejects oversized batch writes, so rows have to be sent in
bounded groups.
"""
import pytest

import receipt_pipeline as rp


def test_splits_into_bounded_groups():
    assert rp.chunk_rows([1, 2, 3, 4, 5], 2) == [[1, 2], [3, 4], [5]]


def test_exact_multiple_leaves_no_empty_group():
    assert rp.chunk_rows([1, 2, 3, 4], 2) == [[1, 2], [3, 4]]


def test_group_larger_than_input_yields_one_group():
    assert rp.chunk_rows([1, 2], 10) == [[1, 2]]


def test_empty_input_yields_no_groups():
    assert rp.chunk_rows([], 3) == []


@pytest.mark.parametrize("size", [0, -1])
def test_invalid_group_size_raises(size):
    # Silently defaulting would produce an infinite loop or a single huge write.
    with pytest.raises(ValueError):
        rp.chunk_rows([1, 2, 3], size)
