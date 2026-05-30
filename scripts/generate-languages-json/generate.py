#!/usr/bin/env python3
"""
Generate languages.json from nvim-treesitter highlights.scm files.
Extracts node types captured as @function, @method, @constructor, @type, @class, etc.
"""
import json
import os
import re
import sys

QUERIES_DIR = sys.argv[1] if len(sys.argv) > 1 else "/home/alejandro/Descargas/nvim-treesitter-main/runtime/queries"

# Default test patterns applied to ALL languages (unless overridden in TEST_PATTERNS).
# Most projects follow test_algo or algo_test conventions.
DEFAULT_TEST_PATTERNS = [
    {"type": "prefix", "value": "test_"},
    {"type": "suffix", "value": "_test"},
    {"type": "suffix", "value": "Test"},
    {"type": "suffix", "value": "Tests"},
    {"type": "suffix", "value": ".spec"},
    {"type": "suffix", "value": ".test"},
]

# Categories: which capture suffixes map to our Functions vs Types
FUNCTION_CAPTURES = {
    'function', 'function.call', 'function.builtin', 'function.macro',
    'function.method', 'function.method.call',
    'method', 'method.call',
    'constructor',
}

TYPE_CAPTURES = {
    'type', 'type.builtin', 'type.definition', 'type.enum', 'type.enum.variant',
    'class', 'class.definition', 'class.builtin',
    'struct', 'struct.definition',
    'interface', 'interface.definition',
    'enum', 'enum.definition', 'enum.variant',
    'module', 'module.builtin',
    'trait', 'trait.definition',
    'impl', 'impl.definition',
}

# Control-flow captures: which highlight captures map to which category
CONTROL_FLOW_CAPTURES = {
    'keyword.conditional': 'branch',
    'keyword.conditional.ternary': 'branch',
    'keyword.repeat': 'loop',
    'keyword.return': 'return',
    'keyword.exception': 'error',
}

# Canonical language name mapping (nvim-treesitter name → our name)
LANG_MAP = {
    'c_sharp': 'C#', 'cpp': 'C++', 'c': 'C',
    'javascript': 'JavaScript', 'typescript': 'TypeScript',
    'python': 'Python', 'go': 'Go', 'rust': 'Rust',
    'java': 'Java', 'php': 'PHP', 'ruby': 'Ruby',
    'swift': 'Swift', 'kotlin': 'Kotlin', 'dart': 'Dart',
    'scala': 'Scala', 'haskell': 'Haskell', 'elixir': 'Elixir',
    'lua': 'Lua', 'bash': 'Bash', 'vim': 'Vim',
    'html': 'HTML', 'css': 'CSS', 'json': 'JSON',
    'sql': 'SQL', 'zig': 'Zig', 'nix': 'Nix',
    'ocaml': 'OCaml', 'julia': 'Julia', 'r': 'R',
    'elixir': 'Elixir', 'clojure': 'Clojure', 'groovy': 'Groovy',
    'perl': 'Perl', 'fish': 'Fish', 'dockerfile': 'Dockerfile',
    'toml': 'TOML', 'yaml': 'YAML', 'xml': 'XML',
    'make': 'Make', 'cmake': 'CMake', 'meson': 'Meson',
    'markdown': 'Markdown', 'latex': 'LaTeX',
    'vue': 'Vue', 'svelte': 'Svelte', 'astro': 'Astro',
    'erlang': 'Erlang', 'fsharp': 'F#', 'elm': 'Elm',
    'purescript': 'PureScript', 'fortran': 'Fortran',
    'objc': 'Objective-C', 'tcl': 'Tcl', 'matlab': 'MATLAB',
    'scheme': 'Scheme', 'racket': 'Racket', 'gdscript': 'GDScript',
    'pascal': 'Pascal', 'vala': 'Vala', 'odin': 'Odin',
    'nim': 'Nim', 'pony': 'Pony', 'haxe': 'Haxe',  # if present
    'verilog': 'Verilog', 'systemverilog': 'SystemVerilog', 'vhdl': 'VHDL',
    'powershell': 'PowerShell', 'graphql': 'GraphQL',
    'terraform': 'Terraform', 'hcl': 'HCL',
    'solidity': 'Solidity', 'thrift': 'Thrift', 'protobuf': 'Protobuf', 'proto': 'Protobuf',
    'ini': 'INI', 'toml': 'TOML', 'dot': 'DOT',
}

# Test pattern definitions for common languages
# Format: canonical_name -> list of test patterns
TEST_PATTERNS = {
    'Go': [
        {"type": "suffix", "value": "_test.go", "same_package": True}
    ],
    'Python': [
        {"type": "prefix", "value": "test_"},
        {"type": "prefix", "value": "test_", "in_dir": "tests/"}
    ],
    'JavaScript': [
        {"type": "suffix", "value": ".test.js"},
        {"type": "suffix", "value": ".spec.js"},
        {"type": "suffix", "value": ".test.ts"},
        {"type": "suffix", "value": ".spec.ts"},
        {"type": "prefix", "value": "test_", "in_dir": "__tests__/"}
    ],
    'TypeScript': [
        {"type": "suffix", "value": ".test.ts"},
        {"type": "suffix", "value": ".spec.ts"},
        {"type": "suffix", "value": ".test.js"},
        {"type": "suffix", "value": ".spec.js"},
        {"type": "prefix", "value": "test_", "in_dir": "__tests__/"}
    ],
    'Java': [
        {"type": "suffix", "value": "Test.java"},
        {"type": "suffix", "value": "Tests.java"}
    ],
    'Ruby': [
        {"type": "suffix", "value": "_spec.rb"},
        {"type": "suffix", "value": "_test.rb"}
    ],
    'Rust': [
        {"type": "suffix", "value": ".rs", "in_dir": "tests/"},
        {"type": "suffix", "value": ".rs", "in_dir": "src/tests/"}
    ],
    'PHP': [
        {"type": "suffix", "value": "Test.php"},
        {"type": "prefix", "value": "Test"}
    ],
    'C#': [
        {"type": "suffix", "value": "Tests.cs"}
    ],
    'C': [
        {"type": "prefix", "value": "test_"},
        {"type": "suffix", "value": "_test.c"},
        {"type": "suffix", "value": ".c", "in_dir": "tests/"}
    ],
    'C++': [
        {"type": "suffix", "value": "_test.cpp"},
        {"type": "suffix", "value": "_test.cc"}
    ],
    'Swift': [
        {"type": "suffix", "value": "Tests.swift"}
    ],
    'Kotlin': [
        {"type": "suffix", "value": "Test.kt"}
    ],
    'Dart': [
        {"type": "suffix", "value": "_test.dart"}
    ],
    'Scala': [
        {"type": "suffix", "value": "Spec.scala"},
        {"type": "suffix", "value": "Test.scala"}
    ],
    'Haskell': [
        {"type": "suffix", "value": "Spec.hs"},
        {"type": "suffix", "value": "Test.hs"}
    ],
    'Elixir': [
        {"type": "suffix", "value": "_test.exs"}
    ],
    'Clojure': [
        {"type": "suffix", "value": "_test.clj"}
    ],
    'Groovy': [
        {"type": "suffix", "value": "Spec.groovy"},
        {"type": "suffix", "value": "Test.groovy"}
    ],
    'Perl': [
        {"type": "suffix", "value": ".t"}
    ],
    'Lua': [
        {"type": "suffix", "value": "_spec.lua"},
        {"type": "suffix", "value": "_test.lua"}
    ],
    'Bash': [
        {"type": "suffix", "value": ".bats"},
        {"type": "suffix", "value": ".test.sh"}
    ],
    'PowerShell': [
        {"type": "suffix", "value": ".Tests.ps1"}
    ],
    'R': [
        {"type": "suffix", "value": "_test.R"}
    ],
    'MATLAB': [
        {"type": "prefix", "value": "test_"}
    ],
    'Julia': [
        {"type": "suffix", "value": "_test.jl"}
    ],
    'Zig': [
        {"type": "suffix", "value": "_test.zig"}
    ],
    'Nim': [
        {"type": "suffix", "value": "_test.nim"}
    ],
    'V': [
        {"type": "suffix", "value": "_test.v"}
    ],
    'Crystal': [
        {"type": "suffix", "value": "_spec.cr"}
    ],
    'D': [
        {"type": "suffix", "value": "_test.d"}
    ],
    'F#': [
        {"type": "suffix", "value": "Tests.fs"}
    ],
    'OCaml': [
        {"type": "suffix", "value": "_test.ml"}
    ],
    'Elm': [
        {"type": "suffix", "value": "Tests.elm"}
    ],
    'ReasonML': [
        {"type": "suffix", "value": "_test.re"}
    ],
    'Purescript': [
        {"type": "suffix", "value": "Spec.purs"}
    ]
}


def canonical_name(dirname):
    """Map nvim-treesitter directory name to canonical language name."""
    if dirname in LANG_MAP:
        return LANG_MAP[dirname]
    # Default: title-case the directory name
    return dirname.replace('_', ' ').title()


def extract_node_types(dirpath, queries_base):
    """Extract node type names from a highlights.scm file, following inherits: chain."""
    functions = set()
    types = set()
    visited = set()
    
    _extract_recursive(dirpath, queries_base, functions, types, visited)
    return functions, types


def _extract_recursive(dirpath, queries_base, functions, types, visited):
    """Recursive extraction following ; inherits: directives."""
    dirname = os.path.basename(dirpath)
    if dirname in visited:
        return
    visited.add(dirname)
    
    filepath = os.path.join(dirpath, 'highlights.scm')
    if not os.path.isfile(filepath):
        return
    
    try:
        with open(filepath, 'r', errors='ignore') as f:
            content = f.read()
    except Exception:
        return
    
    # Check for inherits: before removing comments
    inherits_match = re.search(r';\s*inherits:\s*([a-zA-Z0-9_,\s]+)', content)
    if inherits_match:
        inherited = [p.strip() for p in inherits_match.group(1).split(',')]
        for parent_name in inherited:
            parent_dir = os.path.join(queries_base, parent_name)
            _extract_recursive(parent_dir, queries_base, functions, types, visited)
    
    # Remove comments
    content = re.sub(r';.*$', '', content, flags=re.MULTILINE)
    
    # Find all capture patterns: (something) @capture_name
    pattern = r'\(([a-zA-Z_][a-zA-Z0-9_]*)\b[^)]*\)\s*@([a-zA-Z_.]+)'
    matches = re.findall(pattern, content)
    
    for node_type, capture in matches:
        if capture.startswith('keyword.'):
            continue
        if capture in FUNCTION_CAPTURES:
            functions.add(node_type)
        elif capture in TYPE_CAPTURES:
            types.add(node_type)
    
    # Also handle brackets: [ node_type1 node_type2 ] @capture
    bracket_pattern = r'\[([^\]]+)\]\s*@([a-zA-Z_.]+)'
    bracket_matches = re.findall(bracket_pattern, content)
    
    for nodes_str, capture in bracket_matches:
        if capture.startswith('keyword.'):
            continue
        node_types_list = re.findall(r'([a-zA-Z_][a-zA-Z0-9_]*)', nodes_str)
        if capture in FUNCTION_CAPTURES:
            functions.update(node_types_list)
        elif capture in TYPE_CAPTURES:
            types.update(node_types_list)


def extract_control_flow(dirpath, queries_base):
    """Extract control-flow node types from highlights.scm, following inherits: chain."""
    branch = set()
    loop = set()
    ret = set()
    error = set()
    visited = set()
    
    _extract_control_flow_recursive(dirpath, queries_base, branch, loop, ret, error, visited)
    
    return {
        'branch': branch,
        'loop': loop,
        'return': ret,
        'error': error,
    }


def _extract_control_flow_recursive(dirpath, queries_base, branch, loop, ret, error, visited):
    """Recursive extraction of control-flow captures following ; inherits: directives."""
    dirname = os.path.basename(dirpath)
    if dirname in visited:
        return
    visited.add(dirname)
    
    filepath = os.path.join(dirpath, 'highlights.scm')
    if not os.path.isfile(filepath):
        return
    
    try:
        with open(filepath, 'r', errors='ignore') as f:
            content = f.read()
    except Exception:
        return
    
    # Check for inherits: before removing comments
    inherits_match = re.search(r';\s*inherits:\s*([a-zA-Z0-9_,\s]+)', content)
    if inherits_match:
        inherited = [p.strip() for p in inherits_match.group(1).split(',')]
        for parent_name in inherited:
            parent_dir = os.path.join(queries_base, parent_name)
            _extract_control_flow_recursive(parent_dir, queries_base, branch, loop, ret, error, visited)
    
    # Remove comments
    content = re.sub(r';.*$', '', content, flags=re.MULTILINE)
    
    # Find all capture patterns: (something) @capture_name
    pattern = r'\(([a-zA-Z_][a-zA-Z0-9_]*)\b[^)]*\)\s*@([a-zA-Z_.]+)'
    matches = re.findall(pattern, content)
    
    for node_type, capture in matches:
        if capture in CONTROL_FLOW_CAPTURES:
            category = CONTROL_FLOW_CAPTURES[capture]
            if category == 'branch':
                branch.add(node_type)
            elif category == 'loop':
                loop.add(node_type)
            elif category == 'return':
                ret.add(node_type)
            elif category == 'error':
                error.add(node_type)
    
    # Also handle brackets: [ node_type1 node_type2 ] @capture
    bracket_pattern = r'\[([^\]]+)\]\s*@([a-zA-Z_.]+)'
    bracket_matches = re.findall(bracket_pattern, content)
    
    for nodes_str, capture in bracket_matches:
        if capture in CONTROL_FLOW_CAPTURES:
            category = CONTROL_FLOW_CAPTURES[capture]
            node_types_list = re.findall(r'([a-zA-Z_][a-zA-Z0-9_]*)', nodes_str)
            if category == 'branch':
                branch.update(node_types_list)
            elif category == 'loop':
                loop.update(node_types_list)
            elif category == 'return':
                ret.update(node_types_list)
            elif category == 'error':
                error.update(node_types_list)


def main():
    languages = {}
    skipped = []
    
    for entry in sorted(os.listdir(QUERIES_DIR)):
        dirpath = os.path.join(QUERIES_DIR, entry)
        if not os.path.isdir(dirpath):
            continue
        
        highlights_file = os.path.join(dirpath, 'highlights.scm')
        if not os.path.isfile(highlights_file):
            # Some languages might only have other query files
            skipped.append(entry)
            continue
        
        functions, types = extract_node_types(dirpath, QUERIES_DIR)
        
        if not functions and not types:
            skipped.append(f"{entry} (empty)")
            continue
        
        name = canonical_name(entry)
        # Merge defaults with language-specific overrides
        patterns = list(DEFAULT_TEST_PATTERNS)
        if name in TEST_PATTERNS:
            patterns = TEST_PATTERNS[name]
        languages[name] = {
            "functions": sorted(functions),
            "types": sorted(types),
            "test_patterns": patterns,
        }
        
        # Extract control_flow if present
        cf = extract_control_flow(dirpath, QUERIES_DIR)
        control_flow = {}
        if cf['branch']:
            control_flow['branch'] = sorted(cf['branch'])
        if cf['loop']:
            control_flow['loop'] = sorted(cf['loop'])
        if cf['return']:
            control_flow['return'] = sorted(cf['return'])
        if cf['error']:
            control_flow['error'] = sorted(cf['error'])
        if control_flow:
            languages[name]['control_flow'] = control_flow
    
    output = {"languages": languages}
    json.dump(output, sys.stdout, indent=2, ensure_ascii=False)
    
    # Stats to stderr
    total_funcs = sum(len(v['functions']) for v in languages.values())
    total_types = sum(len(v['types']) for v in languages.values())
    print(f"Generated {len(languages)} languages — {total_funcs} function nodes, {total_types} type nodes", file=sys.stderr)
    if skipped:
        print(f"Skipped {len(skipped)} dirs", file=sys.stderr)


if __name__ == '__main__':
    main()
