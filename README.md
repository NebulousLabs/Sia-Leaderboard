Sia Leaderboard
===============

This repo contains the code that powers the [Sia Leaderboard](https://leaderboard.sia.tech).

## Prerequisites
Go, npm

## Structure

The backend code that powers the leaderboard is included in the top-level directory. The frontend is written in react and resides in `frontend/`.

## Running

A running `siad` node is a preqrequisite for running a leaderboard, since the leaderboard must verify contracts. Download the applicable [Sia](https://github.com/NebulousLabs/Sia/releases) release and run `siad`.

To run your own leaderboard, clone this repo, then build the frontend by doing the following:

```
cd frontend
npm run build-production
```

Once the frontend is built, compile and run the `Sia-Leaderboard` binary from the root of the repo:

```
go build .
./Sia-Leaderboard
```

The leaderboard will be served on `:8080` and will persist its data to `leaderboard.db`.

## License

The MIT License (MIT)

