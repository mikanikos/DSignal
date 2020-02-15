# DSignal - a decentralized privacy-enhanced Signal protocol

## Overview

See [Presentation](./docs/presentation.pdf) for a general overview of the project and [Report](./docs/report.pdf) for further details regarding motivations and the design and implementation of the whole protocol stack.

## Structure

Built on top of Andrea's Peerster:


- Dewmini was in charge of Decentralized Storage: packages 'storage' and 'secure';


- Sergio was in charge of Signal, the integration with Decentralized storage and the GUI: package 'adssignal', modifications of 'gossiper' for integration and modifications of package 'webserver' for GUI;


- Andrea was in charge of the original Peerster, Whisper and the integration with Signal: package 'whisper', modifications of package 'adssignal' for integration and the other non-already mentioned packages.

## Usage

To run a new instance of the Peerster use:


`go run main.go -UIPort="7000" -GUIPort="8080" -gossipAddr="127.0.0.1:5000" -name="Alice" -peers="127.0.0.1:5001" -rtimer=10 -signal`


In the GUI send a new message by double clicking in another user, write a message and later submit. The second field is optional and used to include the metahash of your the receiver identity that will be searched in Decentraliced Storage to execute X3DH. The metahash of every Peer are displayed in the CLI at start
