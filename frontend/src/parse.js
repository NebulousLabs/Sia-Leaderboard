import { List } from 'immutable'

// parseLeaders takes an array of leaders returned by the leaderboard api and
// returns a parsed List of leaderboard entries.
export const parseLeaders = (leaderArray) =>
	List(leaderArray.map((leader) => {
		const ret = {groups: [], name: leader.name, lastUpdated: new Date(leader.timestamp*1000), size: leader.size}
		console.log(leader)
		if (typeof leader.groups !== 'undefined') {
			ret.groups = leader.groups
		}
		return ret
	}))

