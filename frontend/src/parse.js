import { List } from 'immutable'

// Take a number of bytes and return a sane, human-readable size.
export const readableFilesize = (bytes) => {
	const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
	let readableunit = 'B'
	let readablesize = bytes
	for (const unit in units) {
		if (readablesize < 1000) {
			readableunit = units[unit]
			break
		}
		readablesize /= 1000
	}
	return readablesize.toFixed(2) + ' ' + readableunit
}

// parseLeaders takes an array of leaders returned by the leaderboard api and
// returns a parsed List of leaderboard entries.
export const parseLeaders = (leaderArray) =>
	List(leaderArray.map((leader) => {
		const ret = {groups: [], name: leader.name, lastUpdated: new Date(leader.timestamp*1000), size: leader.size}
		if (typeof leader.groups !== 'undefined' && leader.groups !== null) {
			ret.groups = leader.groups
		}
		return ret
	}))

