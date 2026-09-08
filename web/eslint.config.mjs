import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,

  globalIgnores([

    ".next/**",
    // next.config.ts sets distDir: "dist" — build output, not source.
    "dist/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),

  {
    rules: {
      // A name destructured only to keep it out of a rest spread is used, not
      // dead: `{ node: _node, ...rest }` exists so `node` never reaches the
      // DOM element. Underscore marks that intent; ignoreRestSiblings covers
      // the same pattern written without the prefix.
      "@typescript-eslint/no-unused-vars": [
        "warn",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
          ignoreRestSiblings: true,
        },
      ],
    },
  },
]);

export default eslintConfig;
