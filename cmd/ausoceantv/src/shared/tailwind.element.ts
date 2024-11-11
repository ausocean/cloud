// See Also: https://github.com/butopen/web-components-tailwind-starter-kit

import {LitElement, unsafeCSS} from "lit";

import style from "./tailwind.global.css?inline";

const tailwindElement = unsafeCSS(style);

export const TailwindElement = (style) =>
    class extends LitElement {

        static styles = [tailwindElement, unsafeCSS(style)];

    };
