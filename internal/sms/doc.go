// Package sms provides SMS processing, transliteration, multipart reassembly,
// delivery report tracking, and telephone number normalization.
//
// Key components:
//
//   - Assembler: Reassembles multipart concatenated SMS segments (UDH reference,
//     part index, total parts) with automated TTL eviction of stale partials.
//
//   - Tracker: Associates outgoing message references (TP-MR) with recipient phone
//     numbers to track asynchronous delivery confirmations (+CDS status reports).
//
//   - Transliteration: Converts Cyrillic text to Latin phonetics when configured,
//     enabling standard 160-character 7-bit GSM encoding instead of 70-character UCS-2.
//
//   - Normalization: Sanitizes and converts telephone numbers to consistent E.164 formats.
package sms
