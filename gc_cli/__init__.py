"""
GitCode CLI - Command line tool for GitCode.

This package provides a Python wrapper for the gc binary.
"""

__version__ = "0.14.0"
__author__ = "GitCode CLI Contributors"
__all__ = ["__version__", "main"]

from gc_cli.wrapper import main
