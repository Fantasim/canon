package diag

// TypeArg is a Type argument: its String is the canonical type text; types.Type satisfies it.
type TypeArg interface{ String() string }

// ValueArg is a Value argument: its CanonText is the canonical text form; value.Value
// satisfies it.
type ValueArg interface{ CanonText() string }
