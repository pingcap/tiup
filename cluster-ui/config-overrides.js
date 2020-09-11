const { override, addLessLoader } = require('customize-cra')
const { alias, configPaths } = require('react-app-rewire-alias')

const addAlias = () => (config) => {
  alias({
    ...configPaths('tsconfig.paths.json'),
  })(config)
  return config
}

module.exports = override(
  addLessLoader({
    lessOptions: {
      javascriptEnabled: true,
      modifyVars: { '@primary-color': '#3351ff' },
    },
  }),
  addAlias()
)
