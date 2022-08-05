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
        bytes[128] keys;
        uint256 counter;
        uint256 cur;
    }

    mapping (bytes => State) private kv;
    mapping (string => CircularBuffer) private rootskey;
    uint64 latest_block_number;
    uint64 counter_kv;

    // TODO prefix query

    function set(bytes memory key, bytes memory val, uint64 block_number, string memory prefix) public{
        State memory s;
        s.is_exist = true;
        s.val = val;

        kv[key] = s;
        latest_block_number = block_number;

        if (bytes(prefix).length > 0) {
            rootskey[prefix].keys[rootskey[prefix].cur] = key;
            rootskey[prefix].cur++;
            rootskey[prefix].cur %= 128;
            if (rootskey[prefix].counter < 128) {
                rootskey[prefix].counter++;
            }
        }
    
        counter_kv++;
    }
    
    function get(bytes memory key) public view returns (bool, bytes memory){
        State memory s = kv[key];
        return (s.is_exist, s.val);
    }

    function getroot(string memory prefix) public view returns (bytes[] memory, bytes[] memory) {
        bytes[] memory keys = new bytes[](rootskey[prefix].counter);
        bytes[] memory vals = new bytes[](rootskey[prefix].counter);
        for (uint256 i = 0; i < rootskey[prefix].counter; ++i) {
            uint256 idx = (rootskey[prefix].cur + 128 - i - 1) % 128;
            keys[i] = rootskey[prefix].keys[idx];
            bytes memory k = keys[i];
            vals[i] = kv[k].val;
        }

        return (keys, vals);
    }

    function del(bytes memory key) public{
        kv[key].is_exist = false;
        delete kv[key];
         counter_kv--;
    }
}
