package edit

// judgeOp is op's editability question on res under the edit layer, with the request's ctx.
func (a *applier) judgeOp(res resolution, op Op) *judge {
	j := a.snap.judge(res, op, a.env.EditLayer)
	j.ctx = a.ctx
	return j
}

// amendmentRefusal is the refusal Editable gives inside the edit layer's amendment of the path
// or an ancestor, active or not (W5, W11a): below a load `layer`, below another expression `computed`.
func amendmentRefusal(j *judge) error {
	if e := j.editable(); e.Mode == ModeNone && e.Reason != ReasonNone {
		return &NotEditableError{Reason: e.Reason}
	}
	return nil
}
