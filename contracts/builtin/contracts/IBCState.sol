// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.13;
/*
    Provides support and utilities for contract administration
*/
contract IBCState {
    struct State {
        bool is_exist;
        bytes val;
    }

    struct CircularBuffer{
        bytes[129] keys;
        uint256 counter;
        uint256 cur;
    }

    mapping (bytes => State) private kv;
    mapping (string => CircularBuffer) private rootskey;

    function set(bytes memory key, bytes memory val, uint64 block_number, string memory prefix) public{
        State memory s;
        s.is_exist = true;
        s.val = val;

        kv[key] = s;

        if (bytes(prefix).length > 0) {
            rootskey[prefix].keys[rootskey[prefix].cur] = key;
            rootskey[prefix].cur++;
            rootskey[prefix].cur %= 129;
            if (rootskey[prefix].counter < 129) {
                rootskey[prefix].counter++;
            }
        }
    }
    
    function get(bytes memory key) public view returns (bool, bytes memory){
        State memory s = kv[key];
        return (s.is_exist, s.val);
    }

    function getroot(string memory prefix) public view returns (bytes[] memory, bytes[] memory) {
        bytes[] memory keys = new bytes[](rootskey[prefix].counter);
        bytes[] memory vals = new bytes[](rootskey[prefix].counter);
        uint256 cnt = rootskey[prefix].counter;
        if (cnt > 128) {
            cnt = 128;
        }
        for (uint256 i = 0; i < rootskey[prefix].counter; ++i) {
            uint256 idx = (rootskey[prefix].cur + 129 - i - 1) % 129;
            keys[i] = rootskey[prefix].keys[idx];
            bytes memory k = keys[i];
            vals[i] = kv[k].val;
        }

        return (keys, vals);
    }

    function del(bytes memory key) public{
        if (kv[key].is_exist) {
            kv[key].is_exist = false;
            delete kv[key];
        }
    }

    // seq?
}
