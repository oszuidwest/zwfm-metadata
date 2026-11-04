package formatters

import (
	"strings"
	"testing"

	"zwfm-metadata/core"
)

func TestRDSFormatterCharacters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "keeps Dutch letters", in: "Zoë Brontë - BLØF Citroën", want: "Zoë Brontë - BLØF Citroën"},
		{name: "keeps other table letters", in: "Straße Dvořák Čapek ñ å", want: "Straße Dvořák Čapek ñ å"},
		{name: "keeps every measured character", in: "áàéèíìóòúùÑÇŞßĲâäêëîïôöûüñçşğĳα©‰Ğěňőπ£¹²³±İńűµ¿÷°¼½¾§ÁÀÉÈÍÌ", want: "áàéèíìóòúùÑÇŞßĲâäêëîïôöûüñçşğĳα©‰Ğěňőπ£¹²³±İńűµ¿÷°¼½¾§ÁÀÉÈÍÌ"},
		{name: "keeps every measured character, part two", in: "ÓÒÚÙŘČŠŽĐĿÂÄÊËÎÏÔÖÛÜřčšžđŀÃÅÆŒÝÕØÞŊŔĆŚŹŦãåæœŵýõøþŋŕćśźŧ", want: "ÓÒÚÙŘČŠŽĐĿÂÄÊËÎÏÔÖÛÜřčšžđŀÃÅÆŒÝÕØÞŊŔĆŚŹŦãåæœŵýõøþŋŕćśźŧ"},
		{name: "composes decomposed letters", in: "Cafe\u0301", want: "Café"},
		{name: "transliterates letters outside the table", in: "Łódź Ñandú ą", want: "Lódź Ñandú a"},
		{name: "transliterates letters sent with the wrong code", in: "ð ŷ ı", want: "d y i"},
		{name: "drops what has no mapping", in: "Кино 日本 🎵 end", want: "end"},
		{name: "keeps table symbols", in: "£5 20° §", want: "£5 20° §"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := core.NewStructuredText(&core.Metadata{Title: tt.in})
			new(RDSFormatter).Format(st)
			if st.Title != tt.want {
				t.Errorf("Title = %q, want %q", st.Title, tt.want)
			}
		})
	}
}

func TestRDSFormatterCountsCharacters(t *testing.T) {
	// 57 characters but 107 bytes, so counting bytes would remove the parentheses.
	title := strings.Repeat("é", 50) + " (Live)"
	st := core.NewStructuredText(&core.Metadata{Title: title})
	new(RDSFormatter).Format(st)
	if st.Title != title {
		t.Errorf("Title = %q, want %q", st.Title, title)
	}
}
