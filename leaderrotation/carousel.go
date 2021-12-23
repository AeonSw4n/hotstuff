package leaderrotation

import (
	"math/rand"
	"time"

	"github.com/relab/hotstuff"
	"github.com/relab/hotstuff/consensus"
	"github.com/relab/hotstuff/modules"
)

func init() {
	modules.RegisterModule("carousel", NewCarousel)
}

type carousel struct {
	mods *consensus.Modules
	rnd  *rand.Rand
}

func (c *carousel) InitConsensusModule(mods *consensus.Modules, _ *consensus.OptionsBuilder) {
	c.mods = mods
}

func (c carousel) GetLeader(round consensus.View) hotstuff.ID {
	commitHead := c.mods.Consensus().CommittedBlock()

	if commitHead.QuorumCert().Signature() == nil {
		c.mods.Logger().Debug("in startup; using round-robin")
		return chooseRoundRobin(round, c.mods.Configuration().Len())
	}

	if commitHead.View() != round-consensus.View(c.mods.Consensus().ChainLength()) {
		c.mods.Logger().Debugf("fallback to round-robin (view=%d, commitHead=%d)", round, commitHead.View())
		return chooseRoundRobin(round, c.mods.Configuration().Len())
	}

	c.mods.Logger().Debug("proceeding with carousel")

	var (
		block       = commitHead
		f           = hotstuff.NumFaulty(c.mods.Configuration().Len())
		i           = 0
		lastAuthors = consensus.NewIDSet()
		ok          = true
	)

	for ok && i < f && block != consensus.GetGenesis() {
		lastAuthors.Add(block.Proposer())
		block, ok = c.mods.BlockChain().Get(block.Parent())
		i++
	}

	candidates := make([]hotstuff.ID, 0, c.mods.Configuration().Len()-f)

	commitHead.QuorumCert().Signature().Participants().ForEach(func(i hotstuff.ID) {
		if !lastAuthors.Contains(i) {
			candidates = append(candidates, i)
		}
	})

	leader := candidates[c.rnd.Int()%len(candidates)]
	c.mods.Logger().Debugf("chose id %d", leader)

	return leader
}

// NewCarousel returns a new instance of the Carousel leader-election algorithm.
func NewCarousel() consensus.LeaderRotation {
	return &carousel{
		rnd: rand.New(rand.NewSource(time.Now().Unix())),
	}
}
