import React, { PropTypes } from 'react'

const dataStyle = {
	paddingRight: '15px',
}

const LeaderboardEntry = ({name, size, lastUpdated, groups}) => (
	<tr>
		<td style={dataStyle} id="name">{name}</td>
		<td style={dataStyle} id="numbytes">{size/1e9} GB</td>
		<td style={dataStyle} id="timestamp">{lastUpdated.toString()}</td>
		<td style={dataStyle} id="groups">
			{
			groups.reduce((grouptext, groupname) =>
				grouptext === '' ? groupname : grouptext + ', ' + groupname, '')
			}
		</td>
	</tr>
)

LeaderboardEntry.propTypes = {
	name: PropTypes.string.isRequired,
	size: PropTypes.number.isRequired,
	lastUpdated: PropTypes.instanceOf(Date).isRequired,
	groups: PropTypes.instanceOf(Array).isRequired,
}

export default LeaderboardEntry

