// SPDX-License-Identifier: MIT
pragma solidity 0.8.13;

import "./IBCERC20.sol";

contract ERC20Factory {
    mapping(string => IBCERC20) private _created;
    uint private _count;
    string[] private _tokens;

    address constant admin = 0x000000000000000000000000000000000000f009; // crosschain cosmos contract

    function createERC20(string memory _name, string memory _symbol) internal {
        require(msg.sender == admin, "E02");
        //        require(address(_created[name]) == address(0), "ERC20: token already exists");
        _created[_name] = new IBCERC20(_name, _symbol);
        _tokens.push(_name);
        _count++;
    }

    function allTokens() public view returns (string[] memory outArray_) {
        outArray_ = new string[](_tokens.length);
        for (uint i = 0; i < _tokens.length; i++) {
            outArray_[i] = _tokens[i];
        }
    }

    function allBalances(address owner) public view returns (string[] memory tokens_, uint256[] memory balances_) {
        require(address(owner) != address(0), "ERC20: account does not exist");

        tokens_ = new string[](_count);
        balances_ = new uint256[](_count);
        string memory name;
        for (uint i = 0; i < _tokens.length; i++) {
            name = _tokens[i];
            if(address(_created[name]) != address(0)) {
                tokens_[i] = name;
                balances_[i] = _created[name].balanceOf(owner);
            }
        }
    }

    function getERC20Info(string memory _name) public view returns (bool exist_, uint256 totalsupply_) {
        if(address(_created[_name]) == address(0)) {
            exist_ = false;
            totalsupply_ = 0;
        } else {
            exist_ = true;
            totalsupply_ = _created[_name].totalSupply();
        }
    }

    // todo: add some restriction according to usage's environment
    function mintCoin(string memory name, address account, uint256 amount, string memory symbol) public returns (bool) {
        require(msg.sender == admin, "E02");
    
        // do create a new token when it does not exist
        if(address(_created[name]) == address(0)) {
            createERC20(name, symbol);
        }

        _created[name].mint(account, amount);
        return true;
    }

    function getBalance(string memory name, address account) public view returns (uint256) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        return _created[name].balanceOf(account);
    }

    function getAllowance(string memory name, address owner, address spender) public view returns (uint256) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        return _created[name].allowance(owner, spender);
    }

    function burnCoin(string memory name, address account, uint256 amount) public returns (bool) {
        require(msg.sender == admin, "E02");

        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        _created[name].burn(account, amount);
        return true;
    }

    function transferFrom(string memory name, address sender, address recipient, uint256 amount) public returns (bool) {
        require(msg.sender == sender || msg.sender == admin, "E02");
    
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        _created[name].transferFrom(sender, recipient, amount);
        return true;
    }

    function increaseAllowance(string memory name, address spender) public returns (bool) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        _created[name].increaseAllowance(spender, 1000);
        return true;
    }

    function decreaseAllowance(string memory name, address spender) public returns (bool) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        _created[name].decreaseAllowance(spender, 1000);
        return true;
    }

    function destroyCoin(string memory name) public returns (bool) {
        if(address(_created[name]) == address(0)) {
            return true;
        }
        require(msg.sender == admin, "E02");
        require(_created[name].totalSupply() == 0, "ERC20: total supply is not zero");

        require(msg.sender == admin, "E02");

        delete _created[name];
        _count--;
        //        _tokens.pop(name);    // todo: delete element from _tokens

        return true;
    }

    function totalSupply(string memory name) public view returns (uint256) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        return _created[name].totalSupply();
    }
}
