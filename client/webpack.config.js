const { resolve } = require('path')

module.exports = {
  entry: {
    app: ['./src/index.js']
  },
  output: {
    path: resolve(__dirname, "dist"),
    publicPath: "/assets/",
    filename: "bundle.js"
  },
  module: {
    rules: [
      {
        test: /\.jsx?$/,
        exclude: /(node_modules)/,
        loader: 'babel-loader',
        query: {
          presets: ['@babel/preset-env']
        }
      }
    ]
  },
  devServer: {
    proxy: {
      '/api': {
        target: 'http://localhost:8081'
      }
    }
  }
}
