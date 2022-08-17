// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract AddrToPubkeyMap {
    address[] private _addresses;
    mapping(address => string) private _validators;
    uint private _count;

    // How to make sure _addr and _pubkey are related? There should be some restrictions for register.
    function registerValidator(address _addr, string memory _pubkey) public returns (bool) {
        require(address(_addr) != address(0), "AddrToPubkey: account does not exist");
        //        require(_validators[_addr] != string(0), "AddrToPubkey: account already exists");

        _validators[_addr] = _pubkey;
        _addresses.push(_addr);
        _count++;
        return true;
    }

    function getAllValidators() public view returns (address[] memory addresses_, string[] memory pubkeys_) {
        addresses_ = new address[](_count);
        pubkeys_ = new string[](_count);
        address addr;
        for (uint i = 0; i < _count; i++) {
            addr = _addresses[i];
            addresses_[i] = addr;
            pubkeys_[i] = _validators[addr];
        }
    }

    function getValidator(address _addr) public view returns (string memory pubkey) {
        require(address(_addr) != address(0), "AddrToPubkey: account does not exist");

        return _validators[_addr];
    }
}
