import eslintPluginVue from 'eslint-plugin-vue';

export default [
  ...eslintPluginVue.configs['flat/recommended'],
  {
    rules: {
      'vue/multi-word-component-names': 'off',
      'vue/require-default-prop': 'off',
      'no-unused-vars': 'warn',
      'no-undef': 'warn'
    }
  }
];