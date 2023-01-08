package qpeer

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net"

	lib "github.com/quark-io/go-qPeer/qpeer"
)

// Setup

func greet_setup(conn net.Conn, peerid string) (lib.Init, error) {
	msg, json_err := json.Marshal(lib.Setup(peerid))
	if json_err != nil {
		return lib.Init{}, json_err
	}
	_, write_err := conn.Write(msg)
	if write_err != nil {
		return lib.Init{}, write_err
	}

	buffer := make([]byte, 2048)

	n, read_err := conn.Read(buffer)
	if read_err != nil {
		return lib.Init{}, read_err
	}

	var init lib.Init
	json.Unmarshal(buffer[:n], &init)

	return init, nil

}

func send_key(conn net.Conn, AES_key string, pubkey *rsa.PublicKey) (string, error) {
	enc_AES_key, penc_err := lib.Penc_AES(AES_key, pubkey)
	if penc_err != nil {
		return "", penc_err
	}

	_, write_err := conn.Write([]byte(enc_AES_key))
	if write_err != nil {
		return "", write_err
	}

	buffer := make([]byte, 2048)

	n, read_err := conn.Read(buffer)
	if read_err != nil {
		return "", read_err
	}

	return string(buffer[:n]), nil
}

func send_peerinfo(conn net.Conn, lpeer lib.Lpeer, pubkey_pem string, AES_key string) (string, error) {
	var lpeerinfo lib.Peerinfo
	lpeerinfo.Protocol = lpeer.Protocol
	lpeerinfo.Endpoints = lpeer.Endpoints
	lpeerinfo.RSA_Pubkey = pubkey_pem

	kenc_lpeerinfo, kenc_lpeerinfo_err := lib.Kenc_peerinfo(lpeerinfo, AES_key)
	if kenc_lpeerinfo_err != nil {
		return "", kenc_lpeerinfo_err
	}

	_, write_err := conn.Write([]byte(kenc_lpeerinfo))
	if write_err != nil {
		return "", write_err
	}

	buffer := make([]byte, 1024)

	n, read_err := conn.Read(buffer)
	if read_err != nil {
		return "", write_err
	}

	return string(buffer[:n]), nil
}

func Send_bye(conn net.Conn) error {
	_, write_err := conn.Write([]byte("bye"))
	if write_err != nil {
		return write_err
	}

	return nil
}

func Client_setup(all_peers lib.All_peers, lpeer lib.Lpeer, peerip string, port string, pubkey_pem string) error {
	pubkey, rsa_importing_err := lib.RSA_ImportPubkey(pubkey_pem)
	if rsa_importing_err != nil {
		return rsa_importing_err
	}

	address := string(fmt.Sprintf("%s:%s", peerip, port))
	protocol := "tcp"

	conn, err := net.Dial(protocol, address)
	if err != nil {
		return err
	}
	defer conn.Close()

	init, greet_err := greet_setup(conn, lpeer.Peerid)
	if greet_err != nil {
		return lib.ErrorGreet
	}

	if init.Peerid != lib.Sha1_encrypt(init.Pubkey_pem) {
		return lib.ErrorPeerid
	}

	server_pubkey, rsa_importing_err2 := lib.RSA_ImportPubkey(init.Pubkey_pem)
	if rsa_importing_err2 != nil {
		return rsa_importing_err2
	}

	AES_key := lib.AES_keygen()

	kenc_peerinfo, peerinfo_err := send_key(conn, AES_key, server_pubkey)
	if peerinfo_err != nil {
		return lib.ErrorKencpeerinfo
	}

	peerinfo, kdec_peerinfo_err := lib.Dkenc_peerinfo(kenc_peerinfo, AES_key)
	if kdec_peerinfo_err != nil {
		return kdec_peerinfo_err
	}

	if init.Peerid != lpeer.Peerid {
		var peers_err error
		all_peers, peers_err = lib.Save_peer(init.Peerid, peerinfo, AES_key, pubkey, all_peers)
		if peers_err != nil {
			return peers_err
		}

		lib.Write_peers(all_peers)
	} else {
		return lib.ErrorSamePeerid
	}

	bye, bye_err := send_peerinfo(conn, lpeer, pubkey_pem, AES_key)
	if bye_err != nil || bye != "bye" {
		return lib.ErrorBye
	}

	bye_err = Send_bye(conn)
	if bye_err != nil {
		return lib.ErrorBye
	}

	return nil
}

// Exchange Peers

func greet_exchange_peers(conn net.Conn, peerid string) (string, error) {
	msg, json_err := json.Marshal(lib.Exchange_peers(peerid))
	if json_err != nil {
		return "", json_err
	}
	_, write_err := conn.Write(msg)
	if write_err != nil {
		return "", write_err
	}

	buffer := make([]byte, 2048)

	n, read_err := conn.Read(buffer)
	if read_err != nil {
		return "", read_err
	}

	return string(buffer[:n]), nil
}

func send_dkenc_verify(conn net.Conn, dkenc_verify string) (string, error) {
	_, write_err := conn.Write([]byte(dkenc_verify))
	if write_err != nil {
		return "", write_err
	}

	buffer := make([]byte, 8192)

	n, read_err := conn.Read(buffer)
	if read_err != nil {
		return "", read_err
	}

	return string(buffer[:n]), nil
}

func send_temp_peers(conn net.Conn, privkey *rsa.PrivateKey, peers []lib.Peer, AES_key string) (string, error) {
	temp_peers, temp_peers_err := lib.Return_temp_peers(privkey, peers)
	if temp_peers_err != nil {
		return "", temp_peers_err
	}

	enc_temp_peers, enc_temp_peers_err := lib.Share_temp_peers(temp_peers, AES_key)
	if enc_temp_peers_err != nil {
		return "", temp_peers_err
	}

	_, write_err := conn.Write([]byte(enc_temp_peers))
	if write_err != nil {
		return "", write_err
	}

	buffer := make([]byte, 1024)

	n, read_err := conn.Read(buffer)
	if read_err != nil {
		return "", read_err
	}

	return string(buffer[:n]), nil
}

func Client_exchange_peers(all_peers lib.All_peers, lpeer lib.Lpeer, privkey *rsa.PrivateKey, AES_key string, peerip string, port string) error {

	address := string(fmt.Sprintf("%s:%s", peerip, port))
	protocol := "tcp"

	conn, err := net.Dial(protocol, address)
	if err != nil {
		return err
	}
	defer conn.Close()

	kenc_verify, verify_err := greet_exchange_peers(conn, lpeer.Peerid)
	if verify_err != nil {
		return lib.ErrorVerify
	}
	dkenc_verify, kdec_verify_err := lib.Dkenc_verify(kenc_verify, AES_key)
	if kdec_verify_err != nil {
		return kdec_verify_err
	}

	enc_temp_peers, temp_peers_error := send_dkenc_verify(conn, dkenc_verify)
	if temp_peers_error != nil {
		return lib.ErrorRcvTempPeers
	}
	lib.Save_temp_peers(enc_temp_peers, all_peers, AES_key, lpeer)

	if len(all_peers.Peers) >= 5 {
		bye, bye_err := send_temp_peers(conn, privkey, all_peers.Peers, AES_key)
		if bye_err != nil || bye != "bye" {
			return lib.ErrorBye
		}
	} else {
		bye_err := Send_bye(conn)
		if bye_err != nil {
			return lib.ErrorBye
		}
	}

	return nil
}

// Bootstrap

func send_lpeer(conn net.Conn, kenc_lpeer string) (string, error) {
	_, write_err := conn.Write([]byte(kenc_lpeer))
	if write_err != nil {
		return "", write_err
	}

	buffer := make([]byte, 1024)

	n, read_err := conn.Read(buffer)
	if read_err != nil {
		return "", read_err
	}

	return string(buffer[:n]), nil
}

func Client_bootstrap(all_peers lib.All_peers, lpeer lib.Lpeer, privkey *rsa.PrivateKey, AES_key string, peerip string, port string) error {

	address := string(fmt.Sprintf("%s:%s", peerip, port))
	protocol := "tcp"

	conn, err := net.Dial(protocol, address)
	if err != nil {
		return err
	}
	defer conn.Close()

	kenc_verify, verify_err := greet_exchange_peers(conn, lpeer.Peerid)
	if verify_err != nil {
		return lib.ErrorVerify
	}
	dkenc_verify, kdec_verify := lib.Dkenc_verify(kenc_verify, AES_key)
	if kdec_verify != nil {
		return kdec_verify
	}

	enc_temp_peers, temp_peers_error := send_dkenc_verify(conn, dkenc_verify)
	if temp_peers_error != nil {
		return lib.ErrorRcvTempPeers
	}
	lib.Save_temp_peers(enc_temp_peers, all_peers, AES_key, lpeer)

	kenc_lpeer, kenc_lpeer_err := lib.Kenc_lpeer(lpeer, AES_key)
	if kenc_lpeer_err != nil {
		return kenc_lpeer_err
	}

	bye, bye_err := send_lpeer(conn, kenc_lpeer)

	if bye_err != nil || bye != "bye" { //Improve this with better error handling
		return lib.ErrorBye
	}

	return nil
}

// Ping

func Client_ping(all_peers lib.All_peers, peerid string, privkey *rsa.PrivateKey) error {
	peer, peer_err := lib.Decrypt_peer(peerid, privkey, all_peers.Peers)
	if peer_err != nil {
		return peer_err
	}

	var peerinfo lib.Peerinfo
	json.Unmarshal([]byte(peer.Peerinfo), &peerinfo)

	endpoint := peerinfo.Endpoints.PublicEndpoint

	address := string(fmt.Sprintf("%s:%s", endpoint.Ip, endpoint.Port))
	protocol := "tcp"

	conn, err := net.Dial(protocol, address)
	if err != nil {
		lib.Remove_peer(peer.Peerid, all_peers)
	}
	defer conn.Close()

	lib.Write_peers(all_peers)

	return nil
}

// Getback

func Client_getback(all_peers lib.All_peers, peerid string, privkey *rsa.PrivateKey) error {
	peer, peer_err := lib.Decrypt_peer(peerid, privkey, all_peers.Offline_peers)
	if peer_err != nil {
		return peer_err
	}

	var peerinfo lib.Peerinfo
	json.Unmarshal([]byte(peer.Peerinfo), &peerinfo)

	endpoint := peerinfo.Endpoints.PublicEndpoint

	address := string(fmt.Sprintf("%s:%s", endpoint.Ip, endpoint.Port))
	protocol := "tcp"

	conn, err := net.Dial(protocol, address)
	if err == nil {
		lib.Getback_peer(peer.Peerid, all_peers)
	}
	defer conn.Close()

	lib.Write_peers(all_peers)

	return nil
}
