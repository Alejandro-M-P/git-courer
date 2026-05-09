package domain

// FixtureJSON is a minimal 13-language JSON subset for test injection.
// It matches the old hardcoded languageNodeMapping exactly.
const FixtureJSON = `{"languages": {
  "Go":               {"functions": ["function_declaration","method_declaration"], "types": ["type_declaration","type_spec"]},
  "Python":           {"functions": ["function_definition","async_function_definition"], "types": ["class_definition"]},
  "JavaScript":       {"functions": ["function_declaration","arrow_function","method_definition"], "types": ["class_declaration"]},
  "TypeScript":       {"functions": ["function_declaration","arrow_function","method_definition"], "types": ["class_declaration","interface_declaration","type_alias_declaration"]},
  "Rust":             {"functions": ["function_item"], "types": ["struct_item","trait_item","impl_item","enum_item"]},
  "Java":             {"functions": ["method_declaration","constructor_declaration"], "types": ["class_declaration","interface_declaration","enum_declaration"]},
  "C#":               {"functions": ["method_declaration","constructor_declaration"], "types": ["class_declaration","interface_declaration"]},
  "C++":              {"functions": ["function_definition","method_definition"], "types": ["class_definition","struct_definition"]},
  "PHP":              {"functions": ["function_definition","method_definition"], "types": ["class_definition","interface_declaration"]},
  "Ruby":             {"functions": ["method_definition"], "types": ["class_definition","module_definition"]},
  "Swift":            {"functions": ["function_declaration","method_declaration"], "types": ["class_declaration","struct_declaration","enum_declaration"]},
  "Kotlin":           {"functions": ["function_declaration","method_declaration"], "types": ["class_declaration","interface_declaration"]},
  "Dart":             {"functions": ["function_declaration","method_declaration"], "types": ["class_declaration"]}
}}`
