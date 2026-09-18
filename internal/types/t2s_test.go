package types

import "testing"

func TestToSimplified(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"hello world 123", "hello world 123"},
		{"方案与答案", "方案与答案"},   // already simplified: untouched (no 案→桉 drift)
		{"軟體怎麼安裝", "软体怎么安装"}, // script conversion only: t2s is not tw2sp, so 軟體 stays 软体
		{"請聯繫客服", "请联系客服"},   // 繫→系 needs the character table
		{"一目瞭然", "一目了然"},     // phrase beats the character entry 瞭→了 / 瞭→瞭
		{"上鍊與檔案", "上链与档案"},   // 上鍊→上链 is a TSPhrases entry; 檔案→档案 via TSCharacters
		{"繁體 mixed ASCII 文字", "繁体 mixed ASCII 文字"},
	}
	for _, c := range cases {
		if got := toSimplified(c.in); got != c.want {
			t.Errorf("toSimplified(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToSimplifiedTablesLoaded(t *testing.T) {
	t2sOnce.Do(func() { t2sDict = loadT2S() })
	if t2sDict == nil || len(t2sDict.dict) < 4000 {
		t.Fatalf("expected the embedded OpenCC tables to load, got %d entries", lenOrZero())
	}
	if t2sDict.maxLen < 4 {
		t.Fatalf("phrase table missing: maxLen=%d", t2sDict.maxLen)
	}
}

func lenOrZero() int {
	if t2sDict == nil {
		return 0
	}
	return len(t2sDict.dict)
}
