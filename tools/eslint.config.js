import security from "eslint-plugin-security";

export default [
  {
    files: ["../frontend/js/**/*.js"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: {
        window: "readonly",
        document: "readonly",
        navigator: "readonly",
        fetch: "readonly",
        console: "readonly",
        setTimeout: "readonly",
        localStorage: "readonly",
        sessionStorage: "readonly",
        confirm: "readonly",
        prompt: "readonly",
        alert: "readonly",
        HTMLElement: "readonly",
        Promise: "readonly",
      },
    },
    plugins: { security },
    rules: {
      ...security.configs.recommended.rules,
    },
  },
];
