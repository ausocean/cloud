import { LitElement, unsafeCSS } from "lit";
import style from "./tailwind.global.css";

console.log("Tailwind CSS length:", style?.length); // Debug line

const tailwindElement = unsafeCSS(style);

export const TailwindElement = (style?: string) =>
  class extends LitElement {
    static styles = [tailwindElement, style ? unsafeCSS(style) : []];
  };
