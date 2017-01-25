import ReactDOM from 'react-dom'
import React from 'react'
import GroupLeaderboard from './groupleaderboard.js'
import request from 'superagent'
import { parseLeaders } from './parse.js'

request.get('/leaderboard').end((err, res) => {
	if (err) {
		console.error(err)
	}
	const leaders = parseLeaders(JSON.parse(res.text))

	ReactDOM.render(
		<GroupLeaderboard entries={leaders} />,
		document.getElementById('react-root')
	)
})

