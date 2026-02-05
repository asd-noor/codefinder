package scanner

var Queries = map[string]string{
	"go": `
		(function_declaration name: (identifier) @name) @def
		(method_declaration name: (field_identifier) @name) @def
		(type_declaration (type_spec name: (type_identifier) @name)) @def
	`,
	"python": `
		(function_definition name: (identifier) @name) @def
		(class_definition name: (identifier) @name) @def
	`,
	"javascript": `
		(function_declaration name: (identifier) @name) @def
		(class_declaration name: (identifier) @name) @def
		(method_definition name: (property_identifier) @name) @def
		(variable_declarator name: (identifier) @name) @def
	`,
	"typescript": `
		(function_declaration name: (identifier) @name) @def
		(class_declaration name: (type_identifier) @name) @def
		(method_definition name: (property_identifier) @name) @def
		(interface_declaration name: (type_identifier) @name) @def
		(type_alias_declaration name: (type_identifier) @name) @def
	`,
	"zig": `
		(function_declaration (symbol_declaration name: (identifier) @name)) @def
	`,
	"lua": `
		(function_declaration name: [
			(identifier)
			(dot_index_expression)
			(method_index_expression)
		] @name) @def
		(variable_declaration
			(variable_list
				(variable (identifier) @name))) @def
		(assignment_statement
			(variable_list
				(variable (identifier) @name))) @def
	`,
}
