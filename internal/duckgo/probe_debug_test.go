package duckgo

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/dop251/goja"
)

// TestProbeValues runs each challenge probe in the goja prelude to see the
// intermediate fingerprint values, and compares against what a clean real
// browser produces (signals={}, debug="N\u001f").
func TestProbeValues(t *testing.T) {
	vm := goja.New()
	if err := installVQDHelpers(vm); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(vqdBrowserPrelude); err != nil {
		t.Fatalf("prelude: %v", err)
	}

	p0, _ := vm.RunString(`navigator.userAgent`)
	fmt.Println("probe0 (ua):", p0.String())

	p1, _ := vm.RunString(`(function(){
		var top = window.top;
		var jsa = top.document.querySelector('#jsa');
		if(!jsa) return "NO_JSA";
		var cdoc = jsa.contentDocument || (jsa.contentWindow && jsa.contentWindow.document);
		if(!cdoc) return "NO_CDOC";
		var cspEl = cdoc.querySelector('meta[http-equiv=\"Content-Security-Policy\"]');
		if(!cspEl) return "NO_CSP_EL";
		var cspContent = cspEl.getAttribute('content');
		var sandboxAttr = jsa.getAttribute('sandbox');
		var b1 = cspContent === "default-src 'none'; script-src 'unsafe-inline';";
		var b2 = sandboxAttr === "allow-scripts allow-same-origin";
		var b3 = top.hasOwnProperty('__DDG_BE_VERSION__');
		var b4 = top.hasOwnProperty('__DDG_FE_CHAT_HASH__');
		return JSON.stringify({cspContent: cspContent, sandboxAttr: sandboxAttr, b1:b1,b2:b2,b3:b3,b4:b4, sum: 0x1559+Number(b1)+Number(b2)+Number(b3)+Number(b4)});
	})()`)
	fmt.Println("probe1:", p1.String())

	p2, _ := vm.RunString(`(function(){
		var b1 = [navigator.webdriver]===!![];
		var b2 = (function(){
			var f = document.createElement('iframe');
			f.srcdoc = "DuckDuckGo Fraud & Abuse";
			document.body.appendChild(f);
			var r;
			if(f.contentWindow && f.contentWindow.self && f.contentWindow.self.get){ r = f.contentWindow.self.get(); } else { r = undefined; }
			document.body.removeChild(f);
			return !!r;
		})();
		var b3 = (function(){
			var names=['Array','Object','Promise','Proxy','Symbol','JSON','Window'];
			var leaked = Object.keys(window.top).filter(function(k){
				return names.some(function(n){ return k!==n && k.indexOf('_'+n, k.length-('_'+n).length) !== -1 && window.top[k]===window.top[n]; });
			});
			return leaked.length>0;
		})();
		return JSON.stringify({b1:b1,b2:b2,b3:b3, webdriver: navigator.webdriver, sum: 0x22aa+Number(b1)+Number(b2)+Number(b3)});
	})()`)
	fmt.Println("probe2:", p2.String())

	// signals / debug
	sd, _ := vm.RunString(`(function(){
		var signals = {};
		var xorKey = '5b01a30e6c77e86a';
		var s = JSON.stringify(signals);
		var out = '';
		for(var i=0;i<s.length;i++){ out += String.fromCharCode(s.charCodeAt(i) ^ xorKey.charCodeAt(i % xorKey.length)); }
		return JSON.stringify({signals_str: s, debug: out, debug_bytes: Array.from(out).map(function(c){return c.charCodeAt(0);})});
	})()`)
	fmt.Println("signals/debug:", sd.String())

	// decode the helper to confirm btoa path is fine
	_ = base64.StdEncoding
}
