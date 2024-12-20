/*
minidenticons, MIT License: https://github.com/laurentpayot/minidenticons
*/
const t = 5;
function e(e) {
  return e.split("").reduce((e, n) => (e ^ n.charCodeAt(0)) * -t, t) >>> 2;
}
export function minidenticon(t = "", n = 95, i = 45, s = e) {
  const o = s(t),
    c = (o % 9) * 40;
  return (
    [...Array(t ? 25 : 0)].reduce(
      (t, e, n) =>
        o & (1 << n % 15)
          ? t +
            `<rect x="${n > 14 ? 7 - ~~(n / 5) : ~~(n / 5)}" y="${
              n % 5
            }" width="1" height="1"/>`
          : t,
      `<svg viewBox="-1.5 -1.5 8 8" xmlns="http://www.w3.org/2000/svg" fill="hsl(${c} ${n}% ${i}%)">`
    ) + "</svg>"
  );
}
export const minidenticonSvg = globalThis.customElements?.get(
  "minidenticon-svg"
)
  ? null
  : globalThis.customElements?.define(
      "minidenticon-svg",
      class t extends HTMLElement {
        static observedAttributes = ["username", "saturation", "lightness"];
        static #t = {};
        #e = !1;
        connectedCallback() {
          this.#n(), (this.#e = !0);
        }
        attributeChangedCallback() {
          this.#e && this.#n();
        }
        #n() {
          const e = t.observedAttributes.map(
              (t) => this.getAttribute(t) || void 0
            ),
            n = e.join(",");
          this.innerHTML = t.#t[n] ??= minidenticon(...e);
        }
      }
    );
