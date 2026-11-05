import js from '@eslint/js';
import ts from 'typescript-eslint';
export default ts.config(js.configs.recommended, ...ts.configs.recommended, { languageOptions: { globals: { document: 'readonly', localStorage: 'readonly', fetch: 'readonly', URL: 'readonly', AbortController: 'readonly', AbortSignal: 'readonly', DOMException: 'readonly', HTMLElement: 'readonly', HTMLFormElement: 'readonly', HTMLInputElement: 'readonly', HTMLButtonElement: 'readonly', EventListener: 'readonly', RequestInit: 'readonly', crypto: 'readonly', setTimeout: 'readonly', clearTimeout: 'readonly' } } });
