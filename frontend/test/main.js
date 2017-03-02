import React from 'react'
import { List } from 'immutable'
import { mount } from 'enzyme'
import { jsdom } from 'jsdom'
import { expect } from 'chai'
import Leaderboard from '../src/leaderboard.js'
import { readableFilesize } from '../src/parse.js'

global.document = jsdom('')
global.window = document.defaultView

const testEntries = List([
	{ name: 'testuser1', size: 1e9, lastUpdated: new Date(), groups: ['group1']},
	{ name: 'testuser2', size: 1e9*5, lastUpdated: new Date(), groups: ['group2']},
	{ name: 'testuser3', size: 1e9*2, lastUpdated: new Date(), groups: ['group1']},
	{ name: 'testuser9', size: 1e9*0.5, lastUpdated:  new Date(), groups: ['group1', 'group2']},
	{ name: 'testuser5', size: 1e9*0.2, lastUpdated: new Date(), groups: ['group1']},
])

describe('leaderboard', () => {

	describe('rendering and appearance', () => {
		const leaderboardComponent = mount(<Leaderboard entries={testEntries} groupFilters={[]} />)
		const leaderboardEntryComponents = leaderboardComponent.find('LeaderboardEntry')

		it('renders a div with correct number of entries', () => {
			expect(leaderboardEntryComponents.length).to.equal(testEntries.size)
		})
		it('properly renders entry names', () => {
			leaderboardEntryComponents.forEach((entry, idx) => {
				expect(entry.find('#name').text()).to.equal(testEntries.get(idx).name)
			})
		})
		it('properly renders entry bytes', () => {
			leaderboardEntryComponents.forEach((entry, idx) => {
				const expectedText = readableFilesize(testEntries.get(idx).size)
				expect(entry.find('#numbytes').text()).to.equal(expectedText)
			})
		})
		it('properly renders entry timestamps', () => {
			leaderboardEntryComponents.forEach((entry, idx) => {
				expect(entry.find('#timestamp').text()).to.equal(testEntries.get(idx).lastUpdated.toString())
			})
		})
		it('properly renders entry groups', () => {
			leaderboardEntryComponents.forEach((entry, idx) => {
				let expectedText = ''
				for (const group in testEntries.get(idx).groups) {
					if (expectedText === '') {
						expectedText = expectedText + testEntries.get(idx).groups[group]
					} else {
						expectedText = expectedText + ', ' + testEntries.get(idx).groups[group]
					}
				}
				expect(entry.find('#groups').text()).to.equal(expectedText)
			})
		})
		it('filters 0 byte entries', () => {
			const zerobyteTestEntries = testEntries.push(
				{ name: 'testuser6', size: 0, lastUpdated: new Date(), groups: ['group1']}
			)

			expect(mount(<Leaderboard entries={zerobyteTestEntries} groupFilters={[]} />).find('LeaderboardEntry').length).to.equal(testEntries.size)
		})
	})

	describe('behavior', () => {
		it('filters by group name', () => {
			expect(mount(<Leaderboard entries={testEntries} groupFilters={['group1']} />).find('LeaderboardEntry').length).to.equal(4)
			expect(mount(<Leaderboard entries={testEntries} groupFilters={['group2']} />).find('LeaderboardEntry').length).to.equal(2)
			expect(mount(<Leaderboard entries={testEntries} groupFilters={['group1', 'group2']} />).find('LeaderboardEntry').length).to.equal(testEntries.size)
		})
		it('sorts by uploaded bytes', () => {
			const component = mount(<Leaderboard entries={testEntries} groupFilters={[]} sort="uploaded" />)
			const componentEntries = component.find('LeaderboardEntry')

			let isSorted = true
			for (let i = 0; i < componentEntries.length; i++) {
				if (i === 0) {
					continue
				}
				if (componentEntries.at(i).props().size >= componentEntries.at(i-1).props().size) {
					isSorted = false
				}
			}

			expect(isSorted).to.equal(true)
		})
	})
})

