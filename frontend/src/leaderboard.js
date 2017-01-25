import React, { PropTypes } from 'react'
import LeaderboardEntry from './leaderboardentry.js'
import { List } from 'immutable'

const leaderboardStyle = {
	'textAlign': 'left',
}

const Leaderboard = ({entries, groupFilters, sort}) => {
	let sortedEntries = entries
	if (sort === 'uploaded') {
		sortedEntries = entries.sortBy((entry) => -entry.size)
	} else if (sort === 'timestamp') {
		sortedEntries = entries.sortBy((entry) => entry.lastUpdated)
	}

	return (
		<table className="pure-table pure-table-horizontal" style={leaderboardStyle}>
			<thead>
				<tr>
					<th> Name </th>
					<th> Uploaded </th>
					<th> Submitted </th>
					<th> Groups </th>
				</tr>
			</thead>
			<tbody>
				{
				sortedEntries.filter((entry) => groupFilters.length === 0 ? true : entry.groups.some((group) => groupFilters.includes(group))).map((entry, key) =>
					<LeaderboardEntry
						key={key}
						name={entry.name}
						size={entry.size}
						lastUpdated={entry.lastUpdated}
						groups={entry.groups}
					/>
				)
				}
			</tbody>
		</table>
	)
}

Leaderboard.propTypes = {
	entries: PropTypes.instanceOf(List).isRequired,
	groupFilters: PropTypes.array.isRequired,
	sort: PropTypes.string.isRequired,
}

export default Leaderboard

