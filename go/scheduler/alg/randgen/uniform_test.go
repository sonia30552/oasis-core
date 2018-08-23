package randgen

import (
	"flag"
	"math/rand"
	"testing"

	"github.com/oasislabs/ekiden/go/scheduler/alg/randgen/chisquared"
	"github.com/stretchr/testify/assert"
)

var uniformSeed int64
var uniformNumBuckets int
var uniformNumTrials int

func init() {
	flag.Int64Var(&uniformSeed, "uniform-test-seed", 0, "uniform-test reproducibility seed value")
	flag.IntVar(&uniformNumBuckets, "uniform-test-buckets", 100, "uniform-test chi-square buckets")
	flag.IntVar(&uniformNumTrials, "uniform-test-trials", 1000000, "uniform-test chi-square trials")
}

func TestUniformNew(t *testing.T) {
	assert := assert.New(t)
	handleTestSeed(t.Logf, &uniformSeed, "uniform test")
	r := rand.New(rand.NewSource(uniformSeed))
	_, err := NewUniform(0, r)
	assert.Error(err, "NewUniform with zero elements should fail")
	_, err = NewUniform(-10, r)
	assert.Error(err, "NewUniform with negative elements should fail")
}

func TestUniform(t *testing.T) {
	assert := assert.New(t)
	handleTestSeed(t.Logf, &uniformSeed, "uniform-test")
	critValue, err := chisquared.CriticalValue(uniformNumBuckets-1, 0.999)
	if err != nil {
		panic("uniform-test-buckets-1 must be a degree-of-freedom value for which chi-squared critical value can be looked up")
	}
	t.Logf("Chi-squared critical value = %g\n", critValue)
	u, err := NewUniform(uniformNumBuckets, rand.New(rand.NewSource(uniformSeed)))
	assert.NoError(err, "NewUniform should not have failed")
	buckets := make([]int, uniformNumBuckets)
	for ix := 0; ix < uniformNumTrials; ix++ {
		buckets[u.Generate()]++
	}
	nullHypothesisExpected := float64(uniformNumTrials) / float64(uniformNumBuckets)
	t.Logf("Null hypothesis: expected number of entries = %f\n", nullHypothesisExpected)
	t.Logf("Expected per bucket: %f\n", nullHypothesisExpected)
	chiSquared := 0.0
	for _, v := range buckets {
		diff := float64(v) - nullHypothesisExpected
		diffSquared := diff * diff
		chiSquared += diffSquared
	}
	chiSquared = chiSquared / nullHypothesisExpected
	t.Logf("Chi-squared: %f\n", chiSquared)
	assert.True(chiSquared < critValue, "Chi-squared value %f too large", chiSquared)
}