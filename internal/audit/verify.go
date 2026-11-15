package audit

import "context"

const defaultVerifyBatch = 1000

// Verify checks the next batch of entries against the chain. It remembers
// where it stopped, so a scheduled run only examines what is new, and it
// re-checks the last entry it had already verified, which catches a trimmed
// tail or an edit to the newest entries. Edits deeper in history are found by
// VerifyFull.
//
// Once a break is found it is kept and reported on every run until a person
// resolves it: a tampered log is never quietly forgotten.
func (s *Service) Verify(ctx context.Context, limit int) (VerifyResult, error) {
	if limit < 1 {
		limit = defaultVerifyBatch
	}
	stored, err := s.repo.Verification(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	if stored.BrokenSeq != 0 {
		return alreadyBroken(stored), nil
	}
	br, err := s.checkAnchor(ctx, stored)
	if err != nil {
		return VerifyResult{}, err
	}
	if br != nil {
		return s.settle(ctx, stored, outcome{after: stored, br: br})
	}
	return s.run(ctx, pass{stored: stored, start: stored, batch: limit})
}

// VerifyFull checks the whole log from the start, in batches. It is slower,
// so it runs rarely, and it finds a change anywhere in history.
func (s *Service) VerifyFull(ctx context.Context, batch int) (VerifyResult, error) {
	if batch < 1 {
		batch = defaultVerifyBatch
	}
	stored, err := s.repo.Verification(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	if stored.BrokenSeq != 0 {
		return alreadyBroken(stored), nil
	}
	return s.run(ctx, pass{
		stored: stored, start: VerifyState{VerifiedHash: Genesis}, batch: batch, all: true,
	})
}

func alreadyBroken(st VerifyState) VerifyResult {
	return VerifyResult{
		VerifiedThrough: st.VerifiedSeq,
		Broken:          &Break{Seq: st.BrokenSeq, Reason: st.BrokenReason},
	}
}

// checkAnchor re-reads the last entry that was verified before and makes sure
// it is still there and still intact.
func (s *Service) checkAnchor(ctx context.Context, st VerifyState) (*Break, error) {
	if st.VerifiedSeq == 0 {
		return nil, nil
	}
	rows, err := s.repo.After(ctx, st.VerifiedSeq-1, 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || rows[0].Seq != st.VerifiedSeq ||
		rows[0].Hash != st.VerifiedHash || Recompute(rows[0]) != rows[0].Hash {
		return &Break{
			Seq:    st.VerifiedSeq,
			Reason: "an entry that was verified before is missing or was changed",
		}, nil
	}
	return nil, nil
}

// pass describes one verification run.
type pass struct {
	stored VerifyState // what was saved before this run
	start  VerifyState // where this run begins; the genesis for a full pass
	batch  int
	all    bool // continue to the end of the log instead of one batch
}

// outcome is what a run found.
type outcome struct {
	after   VerifyState
	checked int
	br      *Break
}

// run walks the chain from p.start. With p.all it continues to the end of the
// log; otherwise it checks a single batch.
func (s *Service) run(ctx context.Context, p pass) (VerifyResult, error) {
	// The head is read first. Entries appended while we work are newer than
	// the snapshot and are checked next time, not mistaken for tampering.
	head, err := s.repo.Head(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	cur, checked := p.start, 0
	for {
		rows, err := s.repo.After(ctx, cur.VerifiedSeq, p.batch)
		if err != nil {
			return VerifyResult{}, err
		}
		next, br := walk(cur, rows)
		cur, checked = next, checked+len(rows)
		if br == nil && len(rows) == p.batch && p.all {
			continue
		}
		if br == nil {
			br = checkHead(cur, head, len(rows) < p.batch)
		}
		return s.settle(ctx, p.stored, outcome{after: cur, checked: checked, br: br})
	}
}

// walk follows the chain from state through rows and stops at the first
// problem, returning the last good position.
func walk(st VerifyState, rows []Record) (VerifyState, *Break) {
	cur := st
	for _, r := range rows {
		switch {
		case r.Seq != cur.VerifiedSeq+1:
			return cur, &Break{Seq: cur.VerifiedSeq + 1, Reason: "an entry is missing"}
		case r.PrevHash != cur.VerifiedHash:
			return cur, &Break{Seq: r.Seq, Reason: "the entry does not follow the previous one"}
		case Recompute(r) != r.Hash:
			return cur, &Break{Seq: r.Seq, Reason: "the entry's content does not match its hash"}
		}
		cur.VerifiedSeq, cur.VerifiedHash = r.Seq, r.Hash
	}
	return cur, nil
}

// checkHead compares the verified position with the head snapshot once the
// whole log has been read. It catches trimmed tails and a forged head.
func checkHead(cur VerifyState, head Head, readAll bool) *Break {
	switch {
	case cur.VerifiedSeq == head.Seq && cur.VerifiedHash != head.Hash:
		return &Break{Seq: head.Seq, Reason: "the head does not match the newest entry"}
	case cur.VerifiedSeq < head.Seq && readAll:
		return &Break{Seq: cur.VerifiedSeq + 1, Reason: "entries are missing at the end of the log"}
	}
	return nil
}

// settle records the outcome and builds the result.
func (s *Service) settle(
	ctx context.Context, stored VerifyState, o outcome,
) (VerifyResult, error) {
	res := VerifyResult{Checked: o.checked, VerifiedThrough: o.after.VerifiedSeq, Broken: o.br}
	after := o.after
	if o.br != nil {
		after.BrokenSeq, after.BrokenReason = o.br.Seq, o.br.Reason
	}
	if after.VerifiedSeq != stored.VerifiedSeq || after.BrokenSeq != stored.BrokenSeq {
		after.At = s.clock.Now()
		if err := s.repo.SaveVerification(ctx, after); err != nil {
			return VerifyResult{}, err
		}
	}
	return res, nil
}
