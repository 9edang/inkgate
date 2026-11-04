package audit

import (
	"testing"

	"github.com/9edang/inkgate/engine/config"
)

func TestAudit_all(t *testing.T) {
	tc := testCase{
		expectedRecommendations: []string{
			"1.1.1",
			"1.1.2",
			"2.1.3",
			"2.1.7",
			"2.1.8",
			"2.1.9",
			"2.2.1",
			"2.2.2",
			"2.2.3",
			"2.2.4",
			"2.3.1",
			"3.1.1",
			// "3.1.2", -- we added service level rate limit
			"3.1.3",
			"3.3.1",
			"3.3.2",
			"3.3.3",
			"3.3.4",
			"4.1.1",
			"4.1.3", // -- we have prometheus and otel metrics
			// "4.2.1", -- opentelemetryis enabled for tracing
			"4.3.1",
			"5.1.1",
			"5.1.2",
			"5.1.3",
			"5.1.4",
			"5.1.5",
			"5.1.6",
			"5.1.7",
			// "5.2.2", -- we added multiple backends to the test to check for multiple unsafe methods
			"7.1.3", // deprecated server plugin basic auth
			"7.1.7", // deprecated client plugin no-redirect
			"7.2.4", // deprecated influx
			"7.3.1", // deprecated TLS private_key and public_key
		},
		levels: []string{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow},
	}
	testAudit(t, tc)
}

func TestAudit_exclude(t *testing.T) {
	tc := testCase{
		expectedRecommendations: []string{
			"2.1.3",
			"2.1.7",
			"2.1.8",
			"2.1.9",
			"2.2.1",
			"2.2.2",
			"2.2.3",
			"2.2.4",
			"2.3.1",
			"3.1.1",
			// "3.1.2", -- add added service level rate limit
			"3.1.3",
			"3.3.1",
			"3.3.2",
			"3.3.3",
			"3.3.4",
			"4.1.1",
			"4.1.3", // -- we have prometheus and otel metrics
			// "4.2.1", -- opentelemetry is enabled for tracing
			"4.3.1",
			"5.1.1",
			"5.1.2",
			"5.1.3",
			"5.1.4",
			"5.1.5",
			"5.1.6",
			"5.1.7",
			// "5.2.2", -- we added multiple backends to the test to check for multiple unsafe methods
			"7.1.3", // deprecated plugin basic-auth
			"7.1.7", // deprecated client plugin no-redirect
			"7.3.1", // deprecated TLS private_key and public_key
		},
		exclude: []string{"1.1.1", "1.1.2", "7.2.4"},
		levels:  []string{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow},
	}
	testAudit(t, tc)
}

func TestAudit_severity(t *testing.T) {
	tc := testCase{
		expectedRecommendations: []string{
			"2.1.3",
			"3.3.4",
		},
		levels: []string{SeverityCritical},
	}
	testAudit(t, tc)
}

type testCase struct {
	expectedRecommendations []string
	exclude                 []string
	levels                  []string
}

func testAudit(t *testing.T, tc testCase) {
	cfg, err := config.NewParser().Parse("./tests/example1.json")
	if err != nil {
		t.Error(err.Error())
	}
	cfg.Normalize()

	result, err := Audit(&cfg, tc.exclude, tc.levels)
	if err != nil {
		t.Error(err)
		return
	}

	if len(result.Recommendations) != len(tc.expectedRecommendations) {
		t.Errorf("wrong number of recommendations. have %d, expected %d", len(result.Recommendations), len(tc.expectedRecommendations))
	}
	for i, id := range tc.expectedRecommendations {
		if i >= len(result.Recommendations) {
			for j := i; j < len(tc.expectedRecommendations); j++ {
				t.Errorf("missing recomendation: %s", tc.expectedRecommendations[j])
			}
			break
		}
		if result.Recommendations[i].Rule != id {
			t.Errorf("unexpected rule %d: %s", i, result.Recommendations[i].Rule)
		}
	}
}
