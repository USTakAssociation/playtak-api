import typescriptEslintEslintPlugin from "@typescript-eslint/eslint-plugin";
import globals from "globals";
import tsParser from "@typescript-eslint/parser";
import path from "node:path";
import { fileURLToPath } from "node:url";
import js from "@eslint/js";
import { FlatCompat } from "@eslint/eslintrc";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const compat = new FlatCompat({
	baseDirectory: __dirname,
	recommendedConfig: js.configs.recommended,
	allConfig: js.configs.all
});

export default [{
	ignores: ["**/.eslintrc.js"],
}, ...compat.extends("plugin:@typescript-eslint/recommended"), {
	plugins: {
		"@typescript-eslint": typescriptEslintEslintPlugin,
	},

	languageOptions: {
		globals: {
			...globals.node,
			...globals.jest,
		},

		parser: tsParser,
		ecmaVersion: 5,
		sourceType: "module",

		parserOptions: {
			project: "tsconfig.json",
			tsconfigRootDir: "/Users/braydonharris/dev/personal-projects/tak/playtak-api",
		},
	},

	rules: {
		"@typescript-eslint/interface-name-prefix": "off",
		"@typescript-eslint/explicit-function-return-type": "off",
		"@typescript-eslint/explicit-module-boundary-types": "off",
		"@typescript-eslint/no-explicit-any": "off",
		"@typescript-eslint/no-unused-expressions": "off",
		"eol-last": "off",
		"brace-style": "off",
		"comma-dangle": "off",
		"comma-spacing": "off",
		eqeqeq: "off",
		indent: "off",
		"key-spacing": "off",
		"keyword-spacing": "off",
		"max-len": "off",
		"no-ex-assign": "off",
		"no-extra-boolean-cast": "off",
		"no-floating-decimal": "off",
		"no-multi-spaces": "off",
		"no-throw-literal": "off",
		"no-unreachable": "off",
		radix: "off",
		"quote-props": "off",
		quotes: "off",
		"space-before-function-paren": "off",
		"space-in-parens": "off",
		"space-infix-ops": "off",
		"space-unary-ops": "off",
		"spaced-comment": "off",
		indent: ["warn", "tab", {
			SwitchCase: 1,

			ignoredNodes: [
				"FunctionExpression > .params[decorators.length > 0]",
				"FunctionExpression > .params > :matches(Decorator, :not(:first-child))",
				"ClassBody.body > PropertyDefinition[decorators.length > 0] > .key",
			],
		}],
	},
}];