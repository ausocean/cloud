import typescript from 'rollup-plugin-typescript2';
import resolve from '@rollup/plugin-node-resolve';
import postcss from 'rollup-plugin-postcss';
import autoprefixer from 'autoprefixer';
import tailwindcss from '@tailwindcss/postcss';  // Changed this line

export default [
  {
    input: 'ts/site-menu.ts',
    output: {
      file: 's/lit/site-menu.js',
      format: 'iife',
      name: 'siteMenu',
      globals: {
        lit: 'lit',
        'lit/decorators.js': 'decorators_js'
      }
    },
    plugins: [
      resolve(),
      typescript()
    ]
  },
  {
    input: 'ts/nav-menu.ts',
    output: {
      file: 's/lit/nav-menu.js',
      format: 'iife',
      name: 'navMenu',
      globals: {
        lit: 'lit',
        'lit/decorators.js': 'decorators_js'
      }
    },
    plugins: [
      resolve(),
      typescript()
    ]
  },
  {
    input: 'ts/header-group.ts',
    output: {
      file: 's/lit/header-group.js',
      format: 'iife',
      name: 'headerGroup',
      globals: {
        lit: 'lit',
        'lit/decorators.js': 'decorators_js'
      }
    },
    plugins: [
      resolve(),
      typescript()
    ]
  },
  {
    input: 'ts/cron-settings.ts',
    output: {
      file: 's/lit/cron-settings.js',
      format: 'iife',
      name: 'cronSettings',
      globals: {
        lit: 'lit',
        'lit/decorators.js': 'decorators_js'
      }
    },
    plugins: [
      resolve(),
      typescript()
    ]
  },{
      input: 'ts/device-settings.ts',
      output: {
        file: 's/lit/device-settings.js',
        format: 'iife',
        name: 'deviceSettings',
        globals: {
          lit: 'lit',
          'lit/decorators.js': 'decorators_js'
        }
      },
      plugins: [
        postcss({
          inject: false, // Don't inject into <head>
          extract: false, // Don't extract to separate file
          modules: false,
          plugins: [
            tailwindcss(),
            autoprefixer()
          ],
          // This is key - it returns the CSS as a string
          to: undefined,
          minimize: true
        }),
        resolve(),
        typescript({
          tsconfig: './tsconfig.json'
        })
      ]
    }
];
