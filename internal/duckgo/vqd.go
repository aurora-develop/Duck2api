package duckgo

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"os"

	"github.com/dop251/goja"
)

const (
	defaultVQDUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"
	defaultVQDStack     = "l@https://duck.ai/dist/duckai-dist/entry.duckai.c9340e95bd2f7fdc3302.js:2:1308110\n"
)

func GenerateVQDHash(vqdHashRequest string) (string, error) {
	if vqdHashRequest == "" {
		return "", errors.New("empty vqd hash request")
	}

	jsBytes, err := base64.StdEncoding.DecodeString(vqdHashRequest)
	if err != nil {
		return "", fmt.Errorf("decode vqd hash request: %w", err)
	}

	payload, err := executeVQDHashScript(string(jsBytes))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString([]byte(payload)), nil
}

func vqdUserAgent() string {
	if value := os.Getenv("X_USER_AGENT"); value != "" {
		return value
	}
	return defaultVQDUserAgent
}

func vqdStack() string {
	if value := os.Getenv("X_VQD_STACK"); value != "" {
		return value
	}
	return defaultVQDStack
}

func executeVQDHashScript(jsCode string) (string, error) {
	vm := goja.New()
	if err := installVQDHelpers(vm); err != nil {
		return "", err
	}
	if _, err := vm.RunString(vqdBrowserPrelude); err != nil {
		return "", fmt.Errorf("initialize vqd browser mock: %w", err)
	}

	value, err := vm.RunString(jsCode)
	if err != nil {
		return "", fmt.Errorf("execute vqd hash script: %s", jsErrorString(vm, err, nil))
	}

	if promise, ok := value.Export().(*goja.Promise); ok {
		switch promise.State() {
		case goja.PromiseStateFulfilled:
			value = promise.Result()
		case goja.PromiseStateRejected:
			return "", fmt.Errorf("vqd hash script rejected: %s", jsErrorString(vm, nil, promise.Result()))
		default:
			return "", errors.New("vqd hash script did not resolve")
		}
	}

	vm.Set("__vqd_result", value)
	payload, err := vm.RunString(vqdResultMutationScript)
	if err != nil {
		return "", fmt.Errorf("encode vqd hash result: %s", jsErrorString(vm, err, nil))
	}
	if goja.IsUndefined(payload) || goja.IsNull(payload) {
		return "", errors.New("vqd hash script returned an empty result")
	}

	return payload.String(), nil
}

func installVQDHelpers(vm *goja.Runtime) error {
	if err := vm.Set("__goVQDUserAgent", vqdUserAgent()); err != nil {
		return fmt.Errorf("install vqd user agent helper: %w", err)
	}
	if err := vm.Set("__goVQDStack", vqdStack()); err != nil {
		return fmt.Errorf("install vqd stack helper: %w", err)
	}
	if err := vm.Set("__goVQDDuration", fmt.Sprintf("%d", 20+rand.Intn(30))); err != nil {
		return fmt.Errorf("install vqd duration helper: %w", err)
	}
	if err := vm.Set("__goSha256Base64", func(value string) string {
		sum := sha256.Sum256([]byte(value))
		return base64.StdEncoding.EncodeToString(sum[:])
	}); err != nil {
		return fmt.Errorf("install vqd sha256 helper: %w", err)
	}
	if err := vm.Set("__goBase64Encode", func(value string) string {
		return base64.StdEncoding.EncodeToString([]byte(value))
	}); err != nil {
		return fmt.Errorf("install vqd btoa helper: %w", err)
	}
	if err := vm.Set("__goBase64Decode", func(value string) (string, error) {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}); err != nil {
		return fmt.Errorf("install vqd atob helper: %w", err)
	}
	return nil
}

func jsErrorString(vm *goja.Runtime, err error, value goja.Value) string {
	if err != nil {
		if exception, ok := err.(*goja.Exception); ok {
			return exception.String()
		}
		return err.Error()
	}
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return "<empty>"
	}
	object := value.ToObject(vm)
	if stack := object.Get("stack"); !goja.IsUndefined(stack) && !goja.IsNull(stack) {
		return stack.String()
	}
	if message := object.Get("message"); !goja.IsUndefined(message) && !goja.IsNull(message) {
		return message.String()
	}
	return value.String()
}

const vqdResultMutationScript = `
(function (result) {
  if (!result || typeof result !== "object") {
    throw new Error("VQD hash script did not return an object");
  }
  if (!Array.isArray(result.client_hashes)) {
    throw new Error("VQD hash script did not return client_hashes");
  }

  result.client_hashes[0] = __goVQDUserAgent;
  result.client_hashes = result.client_hashes.map(function (value) {
    return __goSha256Base64(value);
  });

  if (result.meta && typeof result.meta === "object") {
    result.meta.origin = "https://duck.ai";
    result.meta.stack = __goVQDStack;
    result.meta.duration = __goVQDDuration;
  }

  return JSON.stringify(result);
})(__vqd_result);
`

const vqdBrowserPrelude = `
(function () {
  const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36";
  const originalObjectKeys = Object.keys;
  Object.keys = function (value) {
    if (value === null || value === undefined) {
      return [];
    }
    return originalObjectKeys(value);
  };

  function toByteArray(value) {
    const text = String(value);
    const bytes = [];
    for (let i = 0; i < text.length; i++) {
      bytes.push(text.charCodeAt(i) & 255);
    }
    return bytes;
  }

  function TextEncoder() {}
  TextEncoder.prototype.encode = function (value) {
    const encoded = encodeURIComponent(String(value));
    const bytes = [];
    for (let i = 0; i < encoded.length; i++) {
      if (encoded[i] === "%") {
        bytes.push(parseInt(encoded.slice(i + 1, i + 3), 16));
        i += 2;
      } else {
        bytes.push(encoded.charCodeAt(i));
      }
    }
    return new Uint8Array(bytes);
  };

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

  function HTMLElement() {}
  function HTMLDivElement() {}
  function HTMLLIElement() {}
  function HTMLIFrameElement() {}

  function NodeList(items) {
    const values = items || [];
    for (let i = 0; i < values.length; i++) {
      this[i] = values[i];
    }
    this.length = values.length;
  }
  NodeList.prototype.item = function (index) {
    return this[index] || null;
  };

  function Element(tagName) {
    this.tagName = String(tagName || "").toUpperCase();
    this.children = [];
    this.parentNode = null;
    this.ownerDocument = null;
    this.attributes = {};
    this.innerHTML = "";
    this.textContent = "";
    this.srcdoc = "";
    this.style = {
      cssText: "",
      display: "inline-block",
      getPropertyValue(name) {
        if (String(name).toLowerCase() === "display") {
          return this.display || "inline-block";
        }
        return "";
      }
    };
    this.offsetWidth = 1;
    this.offsetHeight = 1;
    this.scrollHeight = 1;
  }

  Element.prototype.constructor = Element;
  HTMLElement.prototype = Object.create(Element.prototype);
  HTMLElement.prototype.constructor = HTMLElement;
  HTMLDivElement.prototype = Object.create(HTMLElement.prototype);
  HTMLDivElement.prototype.constructor = HTMLDivElement;
  HTMLLIElement.prototype = Object.create(HTMLElement.prototype);
  HTMLLIElement.prototype.constructor = HTMLLIElement;
  HTMLIFrameElement.prototype = Object.create(HTMLElement.prototype);
  HTMLIFrameElement.prototype.constructor = HTMLIFrameElement;

  Element.prototype.appendChild = function (child) {
    child.parentNode = this;
    child.ownerDocument = this.ownerDocument;
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
    if (selector === "*") {
      if (String(this.innerHTML || "") === "<br><div></br><br></div") {
        return new NodeList(new Array(4).fill({}));
      }
      const matches = String(this.innerHTML || "").match(/<[^/!][^>]*>/gi);
      return new NodeList(new Array(matches ? matches.length : 0).fill({}));
    }
    if (selector === "div") {
      const matches = String(this.innerHTML || "").match(/<div\b/gi);
      return new NodeList(new Array(matches ? matches.length : 0).fill({}));
    }
    if (selector === "meta[http-equiv=\"content-security-policy\"]") {
      const matches = this.children.filter(function (child) {
        return String(child.tagName).toLowerCase() === "meta" &&
          String(child.getAttribute("http-equiv")).toLowerCase() === "content-security-policy";
      });
      return new NodeList(matches);
    }
    return new NodeList([]);
  };

  Element.prototype.querySelector = function (selector) {
    const matches = this.querySelectorAll(selector);
    return matches.length > 0 ? matches[0] : null;
  };

  Element.prototype.getAttribute = function (name) {
    return this.attributes[String(name)] || null;
  };

  Element.prototype.setAttribute = function (name, value) {
    this.attributes[String(name)] = String(value);
  };

  Element.prototype.getBoundingClientRect = function () {
    return {
      width: this.offsetWidth || 1,
      height: this.offsetHeight || 1,
      top: 0,
      right: this.offsetWidth || 1,
      bottom: this.offsetHeight || 1,
      left: 0
    };
  };

  function createElement(tagName) {
    tagName = String(tagName || "").toLowerCase();
    let element;
    if (tagName === "div") {
      element = new HTMLDivElement();
    } else if (tagName === "li") {
      element = new HTMLLIElement();
    } else if (tagName === "iframe") {
      element = new HTMLIFrameElement();
    } else {
      element = new Element(tagName);
    }
    Element.call(element, tagName);
    return element;
  }

  function makeDocument() {
    const document = {};
    const documentElement = new Element("html");
    const head = new Element("head");
    const body = new Element("body");

    documentElement.ownerDocument = document;
    head.ownerDocument = document;
    body.ownerDocument = document;
    documentElement.appendChild(head);
    documentElement.appendChild(body);

    Object.assign(document, {
      documentElement,
      head,
      body,
      createElement(tagName) {
        const element = createElement(tagName);
        element.ownerDocument = document;
        return element;
      },
      querySelectorAll(selector) {
        selector = String(selector || "");
        if (selector === "#jsa" && this.__jsa) {
          return new NodeList([this.__jsa]);
        }
        return documentElement.querySelectorAll(selector);
      },
      querySelector(selector) {
        const matches = this.querySelectorAll(selector);
        return matches.length > 0 ? matches[0] : null;
      }
    });

    return document;
  }

  function makeFrameWindow(contentDocument) {
    const frameWindow = {
      Array,
      Promise,
      Proxy,
      Symbol,
      Window,
      document: contentDocument
    };
    frameWindow.self = frameWindow;
    frameWindow.window = frameWindow;
    frameWindow.top = globalThis;
    contentDocument.defaultView = frameWindow;
    return frameWindow;
  }

  const document = makeDocument();
  const contentDocument = makeDocument();
  const cspMeta = contentDocument.createElement("meta");
  cspMeta.setAttribute("http-equiv", "Content-Security-Policy");
  cspMeta.setAttribute("content", "default-src 'none'; script-src 'unsafe-inline';");
  contentDocument.head.appendChild(cspMeta);

  const hashFrame = document.createElement("iframe");
  hashFrame.setAttribute("id", "jsa");
  hashFrame.setAttribute("sandbox", "allow-scripts allow-same-origin");
  hashFrame.style.cssText = "position: absolute; left: -9999px; top: -9999px;";
  hashFrame.contentDocument = contentDocument;
  hashFrame.contentWindow = makeFrameWindow(contentDocument);
  document.__jsa = hashFrame;
  document.body.appendChild(hashFrame);

  Object.defineProperty(globalThis, "window", { value: globalThis, writable: true, configurable: true });
  Object.defineProperty(globalThis, "self", { value: globalThis, writable: true, configurable: true });
  Object.defineProperty(globalThis, "top", { value: globalThis, writable: true, configurable: true });
  Object.defineProperty(globalThis, Symbol.toStringTag, { value: "Window", writable: true, configurable: true });
  Object.defineProperty(globalThis, "__DDG_BE_VERSION__", { value: 1, writable: true, configurable: true });
  Object.defineProperty(globalThis, "__DDG_FE_CHAT_HASH__", { value: 1, writable: true, configurable: true });
  Object.defineProperty(globalThis, "Window", { value: Window, writable: true, configurable: true });
  Object.defineProperty(globalThis, "Element", { value: Element, writable: true, configurable: true });
  Object.defineProperty(globalThis, "HTMLElement", { value: HTMLElement, writable: true, configurable: true });
  Object.defineProperty(globalThis, "HTMLDivElement", { value: HTMLDivElement, writable: true, configurable: true });
  Object.defineProperty(globalThis, "HTMLLIElement", { value: HTMLLIElement, writable: true, configurable: true });
  Object.defineProperty(globalThis, "HTMLIFrameElement", { value: HTMLIFrameElement, writable: true, configurable: true });
  Object.defineProperty(globalThis, "NodeList", { value: NodeList, writable: true, configurable: true });
  Object.defineProperty(globalThis, "TextEncoder", { value: TextEncoder, writable: true, configurable: true });
  Object.defineProperty(globalThis, "navigator", { value: navigator, writable: true, configurable: true });
  Object.defineProperty(globalThis, "document", { value: document, writable: true, configurable: true });
  Object.defineProperty(globalThis, "btoa", { value: function (value) { return __goBase64Encode(String(value)); }, writable: true, configurable: true });
  Object.defineProperty(globalThis, "atob", { value: function (value) { return __goBase64Decode(String(value)); }, writable: true, configurable: true });
  Object.defineProperty(globalThis, "getComputedStyle", {
    value(element) {
      return element && element.style ? element.style : {
        getPropertyValue() {
          return "";
        }
      };
    },
    writable: true,
    configurable: true
  });
})();
`
