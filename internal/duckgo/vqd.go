package duckgo

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

type vqdHashResult struct {
	ServerHashes []string               `json:"server_hashes"`
	ClientHashes []string               `json:"client_hashes"`
	Signals      map[string]interface{} `json:"signals"`
	Meta         map[string]interface{} `json:"meta"`
}

func GenerateVQDHash(vqdHashRequest string) (string, error) {
	if vqdHashRequest == "" {
		return "", errors.New("empty vqd hash request")
	}

	jsBytes, err := base64.StdEncoding.DecodeString(vqdHashRequest)
	if err != nil {
		return "", fmt.Errorf("decode vqd hash request: %w", err)
	}

	hash, err := executeVQDHashScript(string(jsBytes))
	if err != nil {
		return "", err
	}

	for i, value := range hash.ClientHashes {
		sum := sha256.Sum256([]byte(value))
		hash.ClientHashes[i] = base64.StdEncoding.EncodeToString(sum[:])
	}

	if hash.Signals == nil {
		hash.Signals = map[string]interface{}{}
	}
	if hash.Meta == nil {
		hash.Meta = map[string]interface{}{}
	}
	hash.Meta["origin"] = "https://duckduckgo.com"
	hash.Meta["stack"] = ""
	hash.Meta["duration"] = time.Now().Nanosecond()%100 + 1

	payload, err := json.Marshal(hash)
	if err != nil {
		return "", fmt.Errorf("marshal vqd hash result: %w", err)
	}
	return base64.StdEncoding.EncodeToString(payload), nil
}

func executeVQDHashScript(jsCode string) (vqdHashResult, error) {
	vm := goja.New()
	if _, err := vm.RunString(vqdBrowserPrelude); err != nil {
		return vqdHashResult{}, fmt.Errorf("initialize vqd browser mock: %w", err)
	}

	value, err := vm.RunString(jsCode)
	if err != nil {
		return vqdHashResult{}, fmt.Errorf("execute vqd hash script: %w", err)
	}

	if promise, ok := value.Export().(*goja.Promise); ok {
		switch promise.State() {
		case goja.PromiseStateFulfilled:
			value = promise.Result()
		case goja.PromiseStateRejected:
			return vqdHashResult{}, fmt.Errorf("vqd hash script rejected: %s", promise.Result().String())
		default:
			return vqdHashResult{}, errors.New("vqd hash script did not resolve")
		}
	}

	resultObj := value.ToObject(vm)
	return vqdHashResult{
		ServerHashes: exportStringSlice(resultObj.Get("server_hashes")),
		ClientHashes: exportStringSlice(resultObj.Get("client_hashes")),
		Signals:      exportMap(resultObj.Get("signals")),
		Meta:         exportMap(resultObj.Get("meta")),
	}, nil
}

func exportStringSlice(value goja.Value) []string {
	if goja.IsUndefined(value) || goja.IsNull(value) {
		return nil
	}

	values, ok := value.Export().([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func exportMap(value goja.Value) map[string]interface{} {
	if goja.IsUndefined(value) || goja.IsNull(value) {
		return nil
	}

	result, ok := value.Export().(map[string]interface{})
	if !ok {
		return nil
	}
	return result
}

const vqdBrowserPrelude = `
(function () {
  const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36";

  const navigator = {
    userAgent,
    platform: "Win32",
    language: "en-US",
    languages: ["en-US", "en"],
    cookieEnabled: true,
    onLine: true,
    hardwareConcurrency: 4,
    maxTouchPoints: 0,
    vendor: "Google Inc.",
    vendorSub: "",
    productSub: "20030107",
    appName: "Netscape",
    appVersion: userAgent,
    product: "Gecko"
  };

  function Window() {}
  Window.Array = Array;
  Window.Promise = Promise;
  Window.Proxy = Proxy;
  Window.Symbol = Symbol;
  Window.Window = Window;
  Window.__Array = Array;
  Window.__Promise = Promise;
  Window.__Proxy = Proxy;
  Window.__Symbol = Symbol;
  Window.__Window = Window;

  function nativeGet() {}
  nativeGet.toString = function () {
    return "function get() { [native code] }";
  };

  function makeFrameWindow() {
    const frameWindow = {
      Array,
      Promise,
      Proxy,
      Symbol,
      Window,
      get: nativeGet
    };
    frameWindow.self = frameWindow;
    frameWindow.top = globalThis;
    return frameWindow;
  }

  function HTMLElement() {}
  function HTMLDivElement() {}
  function HTMLLIElement() {}
  function HTMLIFrameElement() {}

  function Element(tagName) {
    this.tagName = String(tagName || "").toUpperCase();
    this.children = [];
    this.parentNode = null;
    this.innerHTML = "";
    this.srcdoc = "";
    if (this.tagName === "IFRAME") {
      this.contentWindow = makeFrameWindow();
    }
  }

  Element.prototype = Object.create(HTMLElement.prototype);
  Element.prototype.constructor = Element;
  HTMLDivElement.prototype = Object.create(Element.prototype);
  HTMLDivElement.prototype.constructor = HTMLDivElement;
  HTMLLIElement.prototype = Object.create(Element.prototype);
  HTMLLIElement.prototype.constructor = HTMLLIElement;
  HTMLIFrameElement.prototype = Object.create(Element.prototype);
  HTMLIFrameElement.prototype.constructor = HTMLIFrameElement;

  Element.prototype.appendChild = function (child) {
    child.parentNode = this;
    this.children.push(child);
    return child;
  };

  Element.prototype.removeChild = function (child) {
    const index = this.children.indexOf(child);
    if (index >= 0) {
      this.children.splice(index, 1);
      child.parentNode = null;
    }
    return child;
  };

  Element.prototype.querySelectorAll = function (selector) {
    selector = String(selector || "").toLowerCase();
    if (selector === "div") {
      const matches = String(this.innerHTML || "").match(/<div\b/gi);
      return new Array(matches ? matches.length : 0).fill({});
    }
    return [];
  };

  Element.prototype.querySelector = function (selector) {
    const matches = this.querySelectorAll(selector);
    return matches.length > 0 ? matches[0] : null;
  };

  Element.prototype.getAttribute = function (name) {
    return this.attributes && this.attributes[String(name)] || null;
  };

  Element.prototype.setAttribute = function (name, value) {
    if (!this.attributes) {
      this.attributes = {};
    }
    this.attributes[String(name)] = String(value);
  };

  function createElement(tagName) {
    tagName = String(tagName || "").toLowerCase();
    if (tagName === "div") {
      return new HTMLDivElement();
    }
    if (tagName === "li") {
      return new HTMLLIElement();
    }
    if (tagName === "iframe") {
      return new HTMLIFrameElement();
    }
    return new Element(tagName);
  }

  Object.defineProperty(HTMLDivElement.prototype, "tagName", { value: "DIV", writable: true, configurable: true });
  Object.defineProperty(HTMLLIElement.prototype, "tagName", { value: "LI", writable: true, configurable: true });
  Object.defineProperty(HTMLIFrameElement.prototype, "tagName", { value: "IFRAME", writable: true, configurable: true });
  HTMLDivElement.call = Element.call;
  HTMLLIElement.call = Element.call;
  HTMLIFrameElement.call = Element.call;

  const body = new Element("body");
  const document = {
    body,
    createElement(tagName) {
      const element = createElement(tagName);
      Element.call(element, tagName);
      return element;
    },
    querySelectorAll() {
      return [];
    },
    querySelector() {
      return null;
    }
  };

  const cspMeta = new Element("meta");
  cspMeta.setAttribute("content", "default-src 'none'; script-src 'unsafe-inline';");
  const hashFrame = new HTMLIFrameElement();
  Element.call(hashFrame, "iframe");
  hashFrame.setAttribute("sandbox", "allow-scripts allow-same-origin");
  hashFrame.contentDocument = {
    querySelector(selector) {
      return String(selector) === "meta[http-equiv=\"Content-Security-Policy\"]" ? cspMeta : null;
    }
  };

  const feChatHash = {
    document: {
      querySelector(selector) {
        return String(selector) === "#jsa" ? hashFrame : null;
      }
    },
    __DDG_BE_VERSION__: "duckchat"
  };

  Object.defineProperty(globalThis, "window", { value: globalThis, writable: true, configurable: true });
  Object.defineProperty(globalThis, "self", { value: globalThis, writable: true, configurable: true });
  Object.defineProperty(globalThis, "top", { value: globalThis, writable: true, configurable: true });
  Object.defineProperty(globalThis, "__DDG_FE_CHAT_HASH__", { value: feChatHash, writable: true, configurable: true });
  Object.defineProperty(globalThis, "Window", { value: Window, writable: true, configurable: true });
  Object.defineProperty(globalThis, "Element", { value: Element, writable: true, configurable: true });
  Object.defineProperty(globalThis, "HTMLElement", { value: HTMLElement, writable: true, configurable: true });
  Object.defineProperty(globalThis, "HTMLDivElement", { value: HTMLDivElement, writable: true, configurable: true });
  Object.defineProperty(globalThis, "HTMLLIElement", { value: HTMLLIElement, writable: true, configurable: true });
  Object.defineProperty(globalThis, "HTMLIFrameElement", { value: HTMLIFrameElement, writable: true, configurable: true });
  Object.defineProperty(globalThis, "navigator", { value: navigator, writable: true, configurable: true });
  Object.defineProperty(globalThis, "document", { value: document, writable: true, configurable: true });
})();
`
