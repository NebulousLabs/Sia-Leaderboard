import ReactDOM from 'react-dom'
import React from 'react'
import { List } from 'immutable'
import App from './app.js'
import request from 'superagent'
import { parseLeaders } from './parse.js'

let entries = List()
let render = () => { }

let sort = 'uploaded'
let groupFilters = []

const onSortChange = (e) => {
	sort = e.target.value
	render()
}

const onGroupFilter = (e) => {
	if (e.target.value === 'nofilter') {
		groupFilters = []
	} else {
		groupFilters = [e.target.value]
	}
	render()
}

render = () => {
	ReactDOM.render(
		<App
			entries={entries}
			groupFilters={groupFilters}
			sort={sort}
			onSort={onSortChange}
			onGroupFilter={onGroupFilter}
		/>,
		document.getElementById('react-root')
	)
}

request.get('/leaderboard').end((err, res) => {
	if (err) {
		console.error(err)
	}
	entries = parseLeaders(JSON.parse(res.text))
	render()
})

