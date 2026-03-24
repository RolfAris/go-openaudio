package server

import (
	"encoding/hex"
	"strings"

	cfg "github.com/cometbft/cometbft/config"
	sm "github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/types"
	"go.uber.org/zap"
)

// Validators that were removed from core_validators (via deregistration) but
// whose Power=0 update was never delivered to CometBFT, leaving them as ghost
// validators in the consensus set. Remove them on startup so the CometBFT
// validator set matches the application state.
var staleValidatorAddresses = map[string]bool{
	"56049970FBAD44D540B8BEF6118800433D269049": true,
	"86A2636C1650226B89755828542D529ADB028BC6": true,
	"A5B56BBFA35E2818A915CFAAEA5A0676C8CDB68E": true,
	"B5EF07A27E9A053561C578504F2649E406804E06": true,
	"C8C249813AC90623B86AF281D06E88CA4686D555": true,
}

// removeStaleValidatorsFromCometState opens the CometBFT state DB, checks for
// the hardcoded stale validators, removes them if present, and saves the state.
// This must be called before nm.NewNode() since CometBFT holds a lock on the DB.
func (s *Server) removeStaleValidatorsFromCometState(cometConfig *cfg.Config) error {
	stateDB, err := cfg.DefaultDBProvider(&cfg.DBContext{ID: "state", Config: cometConfig})
	if err != nil {
		return err
	}
	defer stateDB.Close()

	stateStore := sm.NewStore(stateDB, sm.StoreOptions{})
	state, err := stateStore.Load()
	if err != nil {
		return err
	}

	// No state yet (fresh node)
	if state.IsEmpty() {
		s.logger.Info("no comet state found, skipping validator reconciliation")
		return nil
	}

	removed := 0
	for name, valSet := range map[string]*types.ValidatorSet{
		"Validators":     state.Validators,
		"NextValidators": state.NextValidators,
		"LastValidators": state.LastValidators,
	} {
		n, err := removeStaleFromValidatorSet(valSet)
		if err != nil {
			s.logger.Error("failed to remove stale validators from set",
				zap.String("set", name), zap.Error(err))
			return err
		}
		removed += n
	}

	if removed == 0 {
		s.logger.Info("no stale validators found in comet state, nothing to do")
		return nil
	}

	s.logger.Warn("removing stale validators from comet state",
		zap.Int("removed_total", removed),
		zap.Int("validators_after", state.Validators.Size()),
		zap.Int("next_validators_after", state.NextValidators.Size()),
		zap.Int("last_validators_after", state.LastValidators.Size()),
	)

	if err := stateStore.Save(state); err != nil {
		return err
	}

	s.logger.Info("saved updated comet state with stale validators removed")
	return nil
}

// removeStaleFromValidatorSet removes the hardcoded stale validators from a
// ValidatorSet by applying a change set with Power=0. Returns the number of
// validators removed.
func removeStaleFromValidatorSet(valSet *types.ValidatorSet) (int, error) {
	if valSet == nil {
		return 0, nil
	}

	var changes []*types.Validator
	for _, val := range valSet.Validators {
		addr := strings.ToUpper(hex.EncodeToString(val.Address))
		if staleValidatorAddresses[addr] {
			changes = append(changes, &types.Validator{
				Address:     val.Address,
				PubKey:      val.PubKey,
				VotingPower: 0, // Power=0 signals removal
			})
		}
	}

	if len(changes) == 0 {
		return 0, nil
	}

	if err := valSet.UpdateWithChangeSet(changes); err != nil {
		return 0, err
	}

	return len(changes), nil
}
