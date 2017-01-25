'use strict'

const path = require('path')

module.exports = {
	plugins: [
	],
	entry: {
		index: './src/index.js',
		topgroups: './src/groups.js'
	},
	output: {
		path: path.resolve("./dist"),
		filename: '[name].js'
	},
	resolve: {
		root: path.resolve('./node_modules')
	}, 
	resolveLoader: {
		root: path.resolve('./node_modules'),
	},
	node: {
		__dirname: false,
		__filename: false,
	},
	target: 'web',
	module: {
		loaders: [
			{
				test: /\.js?$/,
				loader: 'babel',
				exclude: /node_modules/,
				query: {
					presets: ['react', 'es2015', 'stage-3']
				}
			}
		]
	}
}
