const { override, addLessLoader } = require('customize-cra')
const { alias, configPaths } = require('react-app-rewire-alias')

const addAlias = () => (config) => {
  alias({
    ...configPaths('tsconfig.paths.json'),
  })(config)
  return config
}

const configEslint = () => (config) => {
  const eslintRule = config.module.rules.filter(
    (r) =>
      r.use && r.use.some((u) => u.options && u.options.useEslintrc !== void 0)
  )[0]
  const options = eslintRule.use[0].options
  // options.ignore = true
  // options.ignorePattern = 'lib/client/api/*.ts'

  // To close "The href attribute is required for an anchor to be keyboard accessible" warning
  options.baseConfig.rules = {
    'jsx-a11y/anchor-is-valid': 'off',
  }
  return config
}

module.exports = override(
  addLessLoader({
    lessOptions: {
      javascriptEnabled: true,
      modifyVars: { '@primary-color': '#3351ff' },
    },
  }),
  addAlias(),
  configEslint()
)
