package domain

// LabelType is a semantic label for an entity change.
type LabelType string

const (
	NEW_FUNC     LabelType = "NEW_FUNC"
	MOD_BODY     LabelType = "MOD_BODY"
	MOD_SIG      LabelType = "MOD_SIG"
	DELETED_FUNC LabelType = "DELETED_FUNC"
	NEW_TYPE     LabelType = "NEW_TYPE"
	MOD_TYPE     LabelType = "MOD_TYPE"
	DELETED_TYPE   LabelType = "DELETED_TYPE"
	MOD_BODY_LOGIC  LabelType = "MOD_BODY_LOGIC"
	MOD_BODY_ERROR  LabelType = "MOD_BODY_ERROR"
	MOD_BODY_REORDER LabelType = "MOD_BODY_REORDER"
	MOD_BODY_CALL   LabelType = "MOD_BODY_CALL"
	CHANGED       LabelType = "CHANGED"
)

// Label is a single semantic annotation for a function/type change.
type Label struct {
	Type     LabelType
	Name     string
	File     string
	Line     int
	Breaking bool
}
