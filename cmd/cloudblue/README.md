# CloudBlue

CloudBlue is a subscription service for accessing AusOceans premium streams.

## Development

CloudBlue uses Golang, Vite, Lit, and Tailwind.

Dependencies:

- npm
- golang

To get started, start by installing the node packages defined in package.json
by calling:

```bash
$ npm install
```

### Golang

Golang is used as the backend of the service, and handles the API,
authentication and databasing.

### Vite

Vite is used as the frontend server of CloudBlue. Vite builds, and serves
static files in production, as well as providing a hot-refresh development
environment. To use Vite in development:

```bash
$ npm run dev
```

To learn more about Vite see: [Vite](https://vite.dev).

### Lit Elements

Lit elements are a thin wrapper on web-components and provide a lightweight
way to define reactive and reusable components. AusOcean uses Typescript to
in our Lit Elements to improve type safety.

To Allow for Tailwind styling to work with Lit components, AusOcean uses a
custom class of LitElement called TailwindElement. This layer adds the
ability for the lit element to parse handle external style sheets generated
by Tailwind. The TailwindElement is defined under `src/shared`.

TailwindElements are defined in individual directories to group an element
with its style sheet, these directories should be stored in the src parent
directory. To create the element, create a new typescript file, and a new css
file. Use the following template to get started:

```TSX
import {html} from 'lit';
import {customElement, property} from 'lit/decorators.js';
import {TailwindElement} from '../shared/tailwind.element';

import style from './test.component.scss?inline'; // # See NOTE

@customElement('test-component')
export class TestComponent extends TailwindElement(style) {

  @property()
  name?: string = 'World';

  render() {
    return html`
      <div>{YOUR ELEMENT HERE}<div>
    `;
  }
}
```

NOTE: Import the css file created for this element, ensure the `?inline`
directive is kept. This tells Vite how to handle importing the data.

This element now works with tailwind class names, and will have all the
required css to style as desired.

To learn more about Lit see: [Lit](https://lit.dev/).
To learn more about TailwindElement see: [Tailwind Element](https://github.com/butopen/web-components-tailwind-starter-kit).

### Tailwind

Tailwind is class based styling framework, which makes styling elements easy
without the worry of unexpected cascading issues.

To Learn more about Tailwind see: [Tailwind](https://tailwindcss.com/).

### Prettier

To ensure that CloudBlue files are easy to edit without worrying about formnatting, this directory includes
a `.prettierrc` file which can be used to format according to the same guidelines. Prettier should be installed
if all npm packages have been installed using `npm install`.

To format all files run:

```bash
$ npm run format
```

OR

```bash
$ npm run format:watch
```

to format when new changes are detected.

To only format a specific file or set of files use:

```bash
$ npx prettier -w [Path]
```

This command will format all the files specified by that path.

Many editors also support Prettier as a default formater, and will read the .prettierrc file from the directory and
apply the relevant formatting when formatted. To set this up will change depending on your development environment.
