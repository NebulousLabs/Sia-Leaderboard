import React, { PropTypes } from 'react'
import Leaderboard from './leaderboard.js'
import { List, Set } from 'immutable'

const appStyle = {
}

const App = ({entries, sort, groupFilters, onSort, onGroupFilter}) => {
	// compute the set of groups in the entire dataset
	let groups = Set()
	entries.forEach((entry) => {
		entry.groups.forEach((group) => {
			groups = groups.add(group)
		})
	})

	return (
		<div style={appStyle}>
			<div style={{marginBottom: '2rem', marginTop: '1rem'}}>
				<span> Sort By: </span>
				<select onChange={onSort}>
					<option value="uploaded">Uploaded (highest first)</option>
					<option value="timestamp">Newest</option>
				</select>
				<span> Filter by Group: </span>
				<select onChange={onGroupFilter}>
					<option value="nofilter">No Filter</option>
					{groups.map((group, key) => <option key={key} value={group}>{group}</option>)}
				</select>
			</div>
			<Leaderboard sort={sort} groupFilters={groupFilters} entries={entries} />
		</div>
	)
}

App.propTypes = {
	entries: PropTypes.instanceOf(List).isRequired,
	sort: PropTypes.string.isRequired,
	groupFilters: PropTypes.array.isRequired,
	onSort: PropTypes.func.isRequired,
	onGroupFilter: PropTypes.func.isRequired,
}

export default App
