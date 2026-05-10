#!/usr/bin/env python3
"""Tests for generate.py — control_flow extraction."""
import json
import os
import sys
import tempfile
import unittest

# Import the module under test
sys.path.insert(0, os.path.dirname(__file__))
import generate


class TestExtractControlFlow(unittest.TestCase):
    """Test extract_control_flow extracts the right categories from highlights.scm."""

    def setUp(self):
        """Create a temporary directory with a fake highlights.scm."""
        self.tmpdir = tempfile.mkdtemp()
        self.queries_dir = self.tmpdir

    def _write_highlights(self, lang_name, content):
        """Write a highlights.scm file for a language."""
        lang_dir = os.path.join(self.queries_dir, lang_name)
        os.makedirs(lang_dir, exist_ok=True)
        with open(os.path.join(lang_dir, 'highlights.scm'), 'w') as f:
            f.write(content)

    def test_conditional_to_branch(self):
        """@keyword.conditional captures map to Branch category."""
        self._write_highlights('testlang', '''
(if_statement) @keyword.conditional
(else_clause) @keyword.conditional
(function_declaration) @function
''')
        cf = generate.extract_control_flow(
            os.path.join(self.queries_dir, 'testlang'),
            self.queries_dir
        )
        self.assertIn('if_statement', cf['branch'])
        self.assertIn('else_clause', cf['branch'])
        self.assertEqual(len(cf['loop']), 0)

    def test_repeat_to_loop(self):
        """@keyword.repeat captures map to Loop category."""
        self._write_highlights('testlang', '''
(for_statement) @keyword.repeat
(while_statement) @keyword.repeat
(function_declaration) @function
''')
        cf = generate.extract_control_flow(
            os.path.join(self.queries_dir, 'testlang'),
            self.queries_dir
        )
        self.assertIn('for_statement', cf['loop'])
        self.assertIn('while_statement', cf['loop'])
        self.assertEqual(len(cf['branch']), 0)

    def test_return_category(self):
        """@keyword.return captures map to Return category."""
        self._write_highlights('testlang', '''
(return_statement) @keyword.return
(function_declaration) @function
''')
        cf = generate.extract_control_flow(
            os.path.join(self.queries_dir, 'testlang'),
            self.queries_dir
        )
        self.assertIn('return_statement', cf['return'])
        self.assertEqual(len(cf['branch']), 0)
        self.assertEqual(len(cf['loop']), 0)

    def test_exception_to_error(self):
        """@keyword.exception captures map to Error category."""
        self._write_highlights('testlang', '''
(try_statement) @keyword.exception
(catch_clause) @keyword.exception
(function_declaration) @function
''')
        cf = generate.extract_control_flow(
            os.path.join(self.queries_dir, 'testlang'),
            self.queries_dir
        )
        self.assertIn('try_statement', cf['error'])
        self.assertIn('catch_clause', cf['error'])

    def test_no_control_flow_captures(self):
        """Language with no control flow captures returns empty dict."""
        self._write_highlights('testlang', '''
(function_declaration) @function
(class_declaration) @type
''')
        cf = generate.extract_control_flow(
            os.path.join(self.queries_dir, 'testlang'),
            self.queries_dir
        )
        self.assertEqual(len(cf['branch']), 0)
        self.assertEqual(len(cf['loop']), 0)
        self.assertEqual(len(cf['return']), 0)
        self.assertEqual(len(cf['error']), 0)

    def test_inherits_merges_control_flow(self):
        """Child language inherits control_flow from parent via ; inherits:"""
        # Parent language with branch + loop
        self._write_highlights('parent_lang', '''
(if_statement) @keyword.conditional
(for_statement) @keyword.repeat
(function_declaration) @function
''')
        # Child language with return + exception
        self._write_highlights('child_lang', '''
; inherits: parent_lang
(return_statement) @keyword.return
(try_statement) @keyword.exception
''')
        cf = generate.extract_control_flow(
            os.path.join(self.queries_dir, 'child_lang'),
            self.queries_dir
        )
        # Should merge: branch from parent, return from child
        self.assertIn('if_statement', cf['branch'])
        self.assertIn('return_statement', cf['return'])
        self.assertIn('try_statement', cf['error'])
        self.assertIn('for_statement', cf['loop'])

    def test_no_highlights_file(self):
        """Language without highlights.scm returns empty dict."""
        lang_dir = os.path.join(self.queries_dir, 'empty_lang')
        os.makedirs(lang_dir, exist_ok=True)
        cf = generate.extract_control_flow(lang_dir, self.queries_dir)
        self.assertEqual(len(cf['branch']), 0)
        self.assertEqual(len(cf['loop']), 0)
        self.assertEqual(len(cf['return']), 0)
        self.assertEqual(len(cf['error']), 0)


if __name__ == '__main__':
    unittest.main()