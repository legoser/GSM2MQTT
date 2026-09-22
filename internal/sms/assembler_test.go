package sms

import (
	"testing"
	"time"
)

func TestAssembler_SinglePart(t *testing.T) {
	a := NewAssembler(5 * time.Minute)
	now := time.Now()

	part := IncomingPart{
		From:        "+79991112233",
		Text:        "Single test message",
		Timestamp:   now,
		IsMultipart: false,
		Encoding:    "gsm7",
	}

	assembled, complete := a.AddPart(part)
	if !complete {
		t.Fatal("expected single message to be complete immediately")
	}
	if assembled == nil {
		t.Fatal("expected non-nil assembled SMS")
	}
	if assembled.Text != "Single test message" {
		t.Errorf("expected %q, got %q", "Single test message", assembled.Text)
	}
	if assembled.Segments != 1 {
		t.Errorf("expected 1 segment, got %d", assembled.Segments)
	}
}

func TestAssembler_MultipartInOrder(t *testing.T) {
	a := NewAssembler(5 * time.Minute)
	now := time.Now()

	part1 := IncomingPart{
		From:        "+79991112233",
		Text:        "Part 1: Hello ",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x42,
		PartNumber:  1,
		TotalParts:  2,
		Encoding:    "ucs2",
	}

	part2 := IncomingPart{
		From:        "+79991112233",
		Text:        "Part 2: World!",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x42,
		PartNumber:  2,
		TotalParts:  2,
		Encoding:    "ucs2",
	}

	// First part should not complete
	res1, complete1 := a.AddPart(part1)
	if complete1 || res1 != nil {
		t.Fatalf("part 1 of 2 should not be complete")
	}

	// Second part should complete
	res2, complete2 := a.AddPart(part2)
	if !complete2 || res2 == nil {
		t.Fatalf("part 2 of 2 should complete message")
	}
	expectedText := "Part 1: Hello Part 2: World!"
	if res2.Text != expectedText {
		t.Errorf("expected concatenated %q, got %q", expectedText, res2.Text)
	}
	if res2.Segments != 2 {
		t.Errorf("expected 2 segments, got %d", res2.Segments)
	}
}

func TestAssembler_MultipartOutOfOrder(t *testing.T) {
	a := NewAssembler(5 * time.Minute)
	now := time.Now()

	// 3-part message arriving as part 2, part 1, part 3
	p2 := IncomingPart{
		From:        "+79991112233",
		Text:        "second, ",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x88,
		PartNumber:  2,
		TotalParts:  3,
	}
	p1 := IncomingPart{
		From:        "+79991112233",
		Text:        "First, ",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x88,
		PartNumber:  1,
		TotalParts:  3,
	}
	p3 := IncomingPart{
		From:        "+79991112233",
		Text:        "third.",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x88,
		PartNumber:  3,
		TotalParts:  3,
	}

	if _, ok := a.AddPart(p2); ok {
		t.Fatal("part 2 should not complete")
	}
	if _, ok := a.AddPart(p1); ok {
		t.Fatal("part 1 should not complete")
	}

	res, ok := a.AddPart(p3)
	if !ok || res == nil {
		t.Fatal("part 3 should complete message")
	}

	expected := "First, second, third."
	if res.Text != expected {
		t.Errorf("expected properly ordered %q, got %q", expected, res.Text)
	}
	if res.Segments != 3 {
		t.Errorf("expected 3 segments, got %d", res.Segments)
	}
}

func TestAssembler_InvalidBounds(t *testing.T) {
	a := NewAssembler(5 * time.Minute)
	now := time.Now()

	// Part number 0
	p0 := IncomingPart{
		From:        "+79991112233",
		Text:        "invalid",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x10,
		PartNumber:  0,
		TotalParts:  2,
	}
	if _, ok := a.AddPart(p0); ok {
		t.Errorf("expected part 0 to be rejected")
	}

	// Part number exceeding total parts
	pExceed := IncomingPart{
		From:        "+79991112233",
		Text:        "invalid",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x10,
		PartNumber:  3,
		TotalParts:  2,
	}
	if _, ok := a.AddPart(pExceed); ok {
		t.Errorf("expected part 3 of 2 to be rejected")
	}

	// Total parts too large (> 20)
	pTooLarge := IncomingPart{
		From:        "+79991112233",
		Text:        "invalid",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x10,
		PartNumber:  1,
		TotalParts:  100,
	}
	if _, ok := a.AddPart(pTooLarge); ok {
		t.Errorf("expected total parts 100 to be rejected")
	}
}

func TestAssembler_TTLExpiration(t *testing.T) {
	// Very short TTL
	a := NewAssembler(30 * time.Millisecond)
	now := time.Now()

	p1 := IncomingPart{
		From:        "+79991112233",
		Text:        "Hello ",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x55,
		PartNumber:  1,
		TotalParts:  2,
	}
	if _, ok := a.AddPart(p1); ok {
		t.Fatal("part 1 should not complete")
	}

	// Sleep longer than TTL so p1 expires and gets cleaned up
	time.Sleep(60 * time.Millisecond)

	// Now part 2 arrives after part 1 expired
	p2 := IncomingPart{
		From:        "+79991112233",
		Text:        "World",
		Timestamp:   now,
		IsMultipart: true,
		Reference:   0x55,
		PartNumber:  2,
		TotalParts:  2,
	}

	// Since part 1 expired, message cannot be completed by part 2 alone
	if _, ok := a.AddPart(p2); ok {
		t.Errorf("expected assembly to fail after TTL expired for previous parts")
	}
}
