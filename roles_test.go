package discovery

import (
	"context"
	"testing"
)

func testRole() RoleProfile {
	return RoleProfile{Key: "support-engineer", Title: "Technical Support Engineer", Focus: "Linux and MySQL incident diagnosis", Requirements: []string{"Diagnose production Linux and MySQL issues", "Own customer accounts and renewals"}}
}

func testEvidences() []RoleEvidence {
	return []RoleEvidence{
		{ID: 2, Revision: 3, Title: "Slow-query diagnosis", Situation: "Payroll outage", Actions: "Checked slow query log, used EXPLAIN, suggested index; DBA applied it", OutcomeState: "unknown"},
	}
}

func TestAnalyzeRoleValidatesIdentitiesAndUnmet(t *testing.T) {
	gen := func(_ context.Context, payload, policy string) (string, error) {
		if policy == "" || payload == "" {
			t.Fatal("empty contract")
		}
		return `{"items":[{"evidence_id":2,"evidence_revision":3,"relevant":true,"capabilities":["MySQL slow-query diagnosis"],"gaps":["Production change ownership stayed with DBA"],"explanation":"Directly supports production diagnosis; does not establish account ownership."}],"unmet":["Own customer accounts and renewals"]}`, nil
	}
	got, err := AnalyzeRole(context.Background(), gen, testRole(), testEvidences())
	if err != nil || got.RoleKey != "support-engineer" || !got.Items[0].Relevant || len(got.Unmet) != 1 {
		t.Fatal(got, err)
	}
}

func TestAnalyzeRoleRejectsMismatchedIdentity(t *testing.T) {
	gen := func(context.Context, string, string) (string, error) {
		return `{"items":[{"evidence_id":9,"evidence_revision":3,"relevant":true,"capabilities":["x"],"gaps":[],"explanation":"Swapped identity."}],"unmet":[]}`, nil
	}
	if _, err := AnalyzeRole(context.Background(), gen, testRole(), testEvidences()); err == nil {
		t.Fatal("swapped evidence identity accepted")
	}
}

func TestAnalyzeRoleRejectsUnmetEchoAndCapabilitiesOnIrrelevant(t *testing.T) {
	invented := func(context.Context, string, string) (string, error) {
		return `{"items":[{"evidence_id":2,"evidence_revision":3,"relevant":false,"capabilities":["Account ownership"],"gaps":[],"explanation":"Irrelevant but claims capability."}],"unmet":["Own customer accounts and renewals"]}`, nil
	}
	if _, err := AnalyzeRole(context.Background(), invented, testRole(), testEvidences()); err == nil {
		t.Fatal("capability on irrelevant record accepted")
	}
	offmenu := func(context.Context, string, string) (string, error) {
		return `{"items":[{"evidence_id":2,"evidence_revision":3,"relevant":true,"capabilities":["MySQL diagnosis"],"gaps":[],"explanation":"Relevant."}],"unmet":["Fluent Mandarin"]}`, nil
	}
	if _, err := AnalyzeRole(context.Background(), offmenu, testRole(), testEvidences()); err == nil {
		t.Fatal("off-menu unmet requirement accepted")
	}
}
