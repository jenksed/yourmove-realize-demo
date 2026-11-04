package main

import (
    "strings"
    "testing"
)

// The document shell must not terminate before late-stage recognition CSS.
// A duplicate </style></head><body> sequence causes raw CSS to display in the page.
func TestPageHTMLKeepsRecognitionCSSInsideStyle(t *testing.T) {
    for _, marker := range []string{"</style>", "</head>", "<body>", "<header>", "<main id=\"app\""} {
        if count := strings.Count(pageHTML, marker); count != 1 {
            t.Fatalf("pageHTML contains %d occurrences of %q; want 1", count, marker)
        }
    }
    cssAt := strings.Index(pageHTML, ".recognition.full-bleed{padding:32px;")
    styleEnd := strings.Index(pageHTML, "</style>")
    if cssAt < 0 || styleEnd < 0 || cssAt > styleEnd {
        t.Fatal("recognition CSS must be inside the document stylesheet")
    }
}
