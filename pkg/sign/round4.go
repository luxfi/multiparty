package sign

import (
	"errors"

	"github.com/taurusgroup/cmp-ecdsa/pb"
	"github.com/taurusgroup/cmp-ecdsa/pkg/math/curve"
	"github.com/taurusgroup/cmp-ecdsa/pkg/party"
	"github.com/taurusgroup/cmp-ecdsa/pkg/round"
	zklogstar "github.com/taurusgroup/cmp-ecdsa/pkg/sign/logstar"
)

type round4 struct {
	*round3
	// delta = δ
	delta *curve.Scalar
	// Delta = Δ
	Delta *curve.Point

	// R = [δ⁻¹] Γ
	R *curve.Point
	// r = R|ₓ
	r *curve.Scalar

	abort bool
}

func (round *round4) ProcessMessage(msg *pb.Message) error {
	j := msg.GetFrom()
	partyJ, ok := round.parties[j]
	if !ok {
		return errors.New("sender not registered")
	}
	body := msg.GetSign3()

	Delta, err := body.GetDeltaGroup().Unmarshal()
	if err != nil {
		return err
	}

	zkLogPublic := zklogstar.Public{
		C:      partyJ.K,
		X:      Delta,
		G:      round.Gamma,
		Prover: partyJ.Paillier,
		Aux:    round.thisParty.Pedersen,
	}

	if !zkLogPublic.Verify(round.H.CloneWithID(j), body.ProofLog) {
		return errors.New("zklog r4 failed")
	}

	partyJ.Delta = Delta
	partyJ.delta = body.GetDelta().Unmarshal()

	return partyJ.AddMessage(msg)
}

func (round *round4) GenerateMessages() ([]*pb.Message, error) {
	// δ = ∑ⱼ δⱼ
	round.delta = curve.NewScalar()
	// Δ = ∑ⱼ Δⱼ
	round.Delta = curve.NewIdentityPoint()
	for _, partyJ := range round.parties {
		round.delta.Add(round.delta, partyJ.delta)
		round.Delta.Add(round.Delta, partyJ.Delta)
	}

	// Δ' = [δ]G
	deltaComputed := curve.NewIdentityPoint().ScalarBaseMult(round.delta)
	if deltaComputed.Equal(round.Delta) != 1 {
		round.abort = true
		return round.GenerateMessagesAbort()
	}

	deltaInv := curve.NewScalar().Invert(round.delta)                    // δ⁻¹
	round.R = curve.NewIdentityPoint().ScalarMult(deltaInv, round.Gamma) // R = [δ⁻¹] Γ
	round.r = round.R.X()                                                // r = R|ₓ

	km := curve.NewScalar().SetHash(round.message)
	km.Multiply(km, round.k)

	// σᵢ = rχᵢ + kᵢm
	round.thisParty.sigma = curve.NewScalar().MultiplyAdd(round.r, round.chi, km)

	return []*pb.Message{{
		Type:      pb.MessageType_TypeSign4,
		From:      round.SelfID,
		Broadcast: pb.Broadcast_Basic,
		Content:   &pb.Message_Sign4{Sign4: &pb.Sign4{Sigma: pb.NewScalar(round.thisParty.sigma)}},
	}}, nil
}

func (round *round4) GenerateMessagesAbort() ([]*pb.Message, error) {
	return nil, nil
}

func (round *round4) Finalize() (round.Round, error) {
	if round.abort {
		panic("abort1")
		return &abort1{round}, nil
	}
	return &output{
		round4: round,
	}, nil
}

func (round *round4) MessageType() pb.MessageType {
	return pb.MessageType_TypeSign3
}

func (round *round4) RequiredMessageCount() int {
	return round.S.N() - 1
}

func (round *round4) IsProcessed(id party.ID) bool {
	panic("implement me")
}